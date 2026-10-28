package library

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"rift/server/internal/blob"
)

const MiB = 1 << 20

func TestUploadEncryptDownloadRoundTrip(t *testing.T) {
	e := newEnv(t)
	p := e.newUser(t)
	plain := randomBytes(9*MiB + 5) // two parts: a full 8 MiB part and a short last one

	ticket, err := e.svc.StartUpload(context.Background(), p, startReq(plain, "Meditations"))
	if err != nil {
		t.Fatal(err)
	}
	if ticket.Status != "created" || len(ticket.Parts) != 2 || len(ticket.BookKey) != 32 {
		t.Fatalf("unexpected ticket: status=%s parts=%d keylen=%d", ticket.Status, len(ticket.Parts), len(ticket.BookKey))
	}
	uploadParts(t, ticket, plain)
	book, err := e.svc.CompleteUpload(context.Background(), p, ticket.UploadID)
	if err != nil {
		t.Fatal(err)
	}

	if got := download(t, e, p, book.ID); !bytes.Equal(got, plain) {
		t.Fatal("downloaded file doesn't decrypt back to the original")
	}
	// Completing twice is harmless and returns the same book.
	again, err := e.svc.CompleteUpload(context.Background(), p, ticket.UploadID)
	if err != nil || again.ID != book.ID {
		t.Fatalf("repeat complete: %v", err)
	}
}

func TestPresignedPartRejectsWrongSize(t *testing.T) {
	e := newEnv(t)
	p := e.newUser(t)
	plain := randomBytes(1000)
	ticket, err := e.svc.StartUpload(context.Background(), p, startReq(plain, "Small"))
	if err != nil {
		t.Fatal(err)
	}
	part := ticket.Parts[0]
	// One byte more than the URL was signed for: storage must refuse it, so a
	// client can't declare a tiny book for quota purposes and upload more.
	status, err := putPart(part.URL, randomBytes(int(part.Size)+1))
	if err != nil {
		t.Fatal(err)
	}
	if status == http.StatusOK {
		t.Fatal("storage accepted a part larger than its signed Content-Length")
	}
}

func TestResumeUploadsOnlyMissingParts(t *testing.T) {
	e := newEnv(t)
	p := e.newUser(t)
	ctx := context.Background()
	plain := randomBytes(17 * MiB) // three parts

	first, err := e.svc.StartUpload(ctx, p, startReq(plain, "Middlemarch"))
	if err != nil {
		t.Fatal(err)
	}
	uploadParts(t, first, plain, 1, 3) // connection drops after parts 1 and 3

	if _, err := e.svc.CompleteUpload(ctx, p, first.UploadID); err == nil {
		t.Fatal("completing with a missing part must fail")
	} else {
		var inc *IncompleteError
		if !errors.As(err, &inc) || len(inc.Missing) != 1 || inc.Missing[0] != 2 {
			t.Fatalf("want missing [2], got %v", err)
		}
	}

	// Same file again (e.g. after an app restart): the upload resumes.
	resumed, err := e.svc.StartUpload(ctx, p, startReq(plain, "Middlemarch"))
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Status != "resuming" || resumed.UploadID != first.UploadID || len(resumed.Parts) != 1 || resumed.Parts[0].Number != 2 {
		t.Fatalf("resume should ask for part 2 only: %+v", resumed)
	}
	uploadParts(t, resumed, plain)
	book, err := e.svc.CompleteUpload(ctx, p, first.UploadID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(download(t, e, p, book.ID), plain) {
		t.Fatal("resumed upload doesn't round-trip")
	}
}

func TestSameFileIsOneBook(t *testing.T) {
	e := newEnv(t)
	p := e.newUser(t)
	plain := randomBytes(2000)
	book := e.addBook(t, p, plain, "Walden")

	// The same file from another device is recognised, not uploaded again.
	other := p
	other.DeviceID = e.newDevice(t, p.UserID, "Windows PC")
	ticket, err := e.svc.StartUpload(context.Background(), other, startReq(plain, "Walden (copy)"))
	if err != nil {
		t.Fatal(err)
	}
	if ticket.Status != "exists" || ticket.Book.ID != book.ID || len(ticket.Parts) != 0 {
		t.Fatalf("duplicate should be recognised: %+v", ticket)
	}
}

func TestQuotaHoldsUnderConcurrentUploads(t *testing.T) {
	e := newEnv(t)
	p := e.newUser(t)
	ctx := context.Background()

	// Five 45 MB books started at once against a 200 MB quota: exactly four fit.
	var ok, refused atomic.Int32
	var wg sync.WaitGroup
	for i := range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := startReq(randomBytes(64), "Big")
			req.Size = 45 * 1000 * 1000
			req.SHA256 = req.SHA256[:62] + []string{"00", "01", "02", "03", "04"}[i]
			_, err := e.svc.StartUpload(ctx, p, req)
			var q *QuotaError
			switch {
			case err == nil:
				ok.Add(1)
			case errors.As(err, &q):
				refused.Add(1)
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != 4 || refused.Load() != 1 {
		t.Fatalf("ok=%d refused=%d, want 4 and 1", ok.Load(), refused.Load())
	}
}

func TestAbortAndStaleCleanupFreeSpace(t *testing.T) {
	e := newEnv(t)
	p := e.newUser(t)
	ctx := context.Background()

	t1, err := e.svc.StartUpload(ctx, p, startReq(randomBytes(3000), "One"))
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.AbortUpload(ctx, p, t1.UploadID); err != nil {
		t.Fatal(err)
	}
	if used, _ := e.q.UserUsedBytes(ctx, p.UserID); used != 0 {
		t.Fatalf("aborted upload still holds %d bytes", used)
	}

	t2, err := e.svc.StartUpload(ctx, p, startReq(randomBytes(4000), "Two"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.pool.Exec(ctx, "UPDATE uploads SET created_at = now() - interval '2 days'"); err != nil {
		t.Fatal(err)
	}
	n, err := e.svc.CleanupStaleUploads(ctx, 24*time.Hour)
	if err != nil || n != 1 {
		t.Fatalf("cleanup removed %d uploads, err %v", n, err)
	}
	if used, _ := e.q.UserUsedBytes(ctx, p.UserID); used != 0 {
		t.Fatalf("stale upload still holds %d bytes", used)
	}
	if _, err := e.svc.GetUpload(ctx, p, t2.UploadID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cleaned-up upload should be gone, got %v", err)
	}
}

func TestChangeFeedDeltaCompactionAndPaging(t *testing.T) {
	e := newEnv(t)
	p := e.newUser(t)
	ctx := context.Background()

	a := e.addBook(t, p, randomBytes(100), "A")
	e.addBook(t, p, randomBytes(200), "B")
	e.addBook(t, p, randomBytes(300), "C")

	all, err := e.svc.Changes(ctx, p.UserID, 0, 0)
	if err != nil || len(all.Changes) != 3 || all.HasMore {
		t.Fatalf("fresh device should get 3 books: %+v %v", all, err)
	}

	// Ten renames of one book compact into a single newer entry.
	for i := range 10 {
		title := "A, edition " + string(rune('0'+i))
		if _, err := e.svc.UpdateBook(ctx, p, a.ID, BookPatch{Title: &title}); err != nil {
			t.Fatal(err)
		}
	}
	delta, err := e.svc.Changes(ctx, p.UserID, all.Cursor, 0)
	if err != nil || len(delta.Changes) != 1 || delta.Changes[0].ID != a.ID {
		t.Fatalf("delta should hold one compacted entry for A: %+v %v", delta, err)
	}
	fresh, _ := e.svc.Changes(ctx, p.UserID, 0, 0)
	if len(fresh.Changes) != 3 {
		t.Fatalf("a new device still sees exactly 3 entries, got %d", len(fresh.Changes))
	}

	// Paging.
	page1, _ := e.svc.Changes(ctx, p.UserID, 0, 2)
	page2, _ := e.svc.Changes(ctx, p.UserID, page1.Cursor, 2)
	if len(page1.Changes) != 2 || !page1.HasMore || len(page2.Changes) != 1 || page2.HasMore {
		t.Fatalf("paging broken: %d/%v then %d/%v", len(page1.Changes), page1.HasMore, len(page2.Changes), page2.HasMore)
	}

	// Nothing new: the cursor still reaches the latest version.
	idle, _ := e.svc.Changes(ctx, p.UserID, delta.Cursor, 0)
	if len(idle.Changes) != 0 || idle.Cursor != delta.Cursor {
		t.Fatalf("idle poll should be empty at the same cursor: %+v", idle)
	}
}

func TestPositionConflictsResolveOnCorrectedClocks(t *testing.T) {
	e := newEnv(t)
	phone := e.newUser(t)
	pc := phone
	pc.DeviceID = e.newDevice(t, phone.UserID, "Windows PC")
	ctx := context.Background()
	book := e.addBook(t, phone, randomBytes(500), "Dune")

	// Phone reads to page 10 and syncs.
	r1, err := e.svc.RecordPosition(ctx, phone, book.ID, RecordPosition{
		Locator: locator(10), Progress: 0.1, ReadAt: ago(time.Minute), ClientNow: time.Now(),
	})
	if err != nil || !r1.Applied {
		t.Fatalf("first position: %+v %v", r1, err)
	}

	// PC saw that version and reads on: fast path, applied.
	r2, err := e.svc.RecordPosition(ctx, pc, book.ID, RecordPosition{
		Locator: locator(30), Progress: 0.3, ReadAt: time.Now(), ClientNow: time.Now(), BaseVersion: r1.Current.Version,
	})
	if err != nil || !r2.Applied || r2.Current.DeviceID != pc.DeviceID {
		t.Fatalf("fast path: %+v %v", r2, err)
	}

	// The phone was offline, read to page 20 an hour ago, and syncs late with a
	// stale base version. Its reading is older than the PC's, so it loses...
	r3, err := e.svc.RecordPosition(ctx, phone, book.ID, RecordPosition{
		Locator: locator(20), Progress: 0.2, ReadAt: ago(time.Hour), ClientNow: time.Now(), BaseVersion: r1.Current.Version,
	})
	if err != nil || r3.Applied || r3.Current.DeviceID != pc.DeviceID {
		t.Fatalf("older offline reading must not win: %+v %v", r3, err)
	}
	// ...but the phone's own latest position is kept (for undo).
	var page int
	if err := e.svc.pool.QueryRow(ctx, "SELECT (locator->>'page')::int FROM positions WHERE book_id=$1 AND device_id=$2", book.ID, phone.DeviceID).Scan(&page); err != nil || page != 20 {
		t.Fatalf("losing write should be on record: page %d, err %v", page, err)
	}

	// A tablet whose clock is 2 hours slow reads "now". Its raw timestamp looks
	// 2 hours old, but corrected by its measured offset it's the newest reading.
	tablet := phone
	tablet.DeviceID = e.newDevice(t, phone.UserID, "Tablet")
	skew := 2 * time.Hour
	r4, err := e.svc.RecordPosition(ctx, tablet, book.ID, RecordPosition{
		Locator: locator(40), Progress: 0.4, ReadAt: time.Now().Add(-skew), ClientNow: time.Now().Add(-skew), BaseVersion: r1.Current.Version,
	})
	if err != nil || !r4.Applied {
		t.Fatalf("skewed clock should be corrected and win: %+v %v", r4, err)
	}
}

func TestCorrectedReadAtNeverInFuture(t *testing.T) {
	server := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	// Device clock 5 minutes fast, claims it read "1 minute ago" by its clock.
	got := CorrectedReadAt(server.Add(4*time.Minute), server.Add(5*time.Minute), server)
	if !got.Equal(server.Add(-time.Minute)) {
		t.Fatalf("got %v, want 1 minute before server time", got)
	}
	// A read_at after its own client_now can't land in the server's future.
	if got := CorrectedReadAt(server.Add(time.Hour), server, server); !got.Equal(server) {
		t.Fatalf("future reading should clamp to server time, got %v", got)
	}
}

func TestTrashRestoreAndPurge(t *testing.T) {
	e := newEnv(t)
	p := e.newUser(t)
	ctx := context.Background()
	book := e.addBook(t, p, randomBytes(1500), "Dracula")
	storageKey := "u/" + p.UserID.String() + "/b/" + book.ID.String()

	if _, err := e.svc.TrashBook(ctx, p, book.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Download(ctx, p, book.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("trashed book shouldn't download, got %v", err)
	}
	if _, err := e.svc.RestoreBook(ctx, p, book.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Download(ctx, p, book.ID); err != nil {
		t.Fatalf("restored book should download: %v", err)
	}

	if _, err := e.svc.TrashBook(ctx, p, book.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.pool.Exec(ctx, "UPDATE books SET trashed_at = now() - interval '31 days'"); err != nil {
		t.Fatal(err)
	}
	before, _ := e.svc.Changes(ctx, p.UserID, 0, 0)
	n, err := e.svc.PurgeTrash(ctx)
	if err != nil || n != 1 {
		t.Fatalf("purged %d, err %v", n, err)
	}

	if _, err := e.svc.blob.Size(ctx, storageKey); !errors.Is(err, blob.ErrNotFound) {
		t.Fatalf("stored file should be deleted, got %v", err)
	}
	if used, _ := e.q.UserUsedBytes(ctx, p.UserID); used != 0 {
		t.Fatalf("purge should free space, %d bytes still used", used)
	}
	after, _ := e.svc.Changes(ctx, p.UserID, before.Cursor, 0)
	ops := map[string]string{}
	for _, c := range after.Changes {
		ops[c.Entity] = c.Op
	}
	if ops["book"] != "deleted" || ops["position"] != "deleted" {
		t.Fatalf("offline devices need tombstones, got %v", ops)
	}
}
