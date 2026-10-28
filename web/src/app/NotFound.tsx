import { ButtonLink } from "../components/Button";
import { RiftMark } from "../components/RiftMark";

export function NotFound() {
  return (
    <main style={{ minHeight: "100dvh", display: "grid", placeItems: "center", padding: "2rem 1rem" }}>
      <div style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: "1rem", textAlign: "center" }}>
        <RiftMark size={40} />
        <h1 style={{ fontSize: "var(--text-2xl)" }}>Nothing at this address</h1>
        <p style={{ color: "var(--text-secondary)" }}>The page may have moved, or the link is incomplete.</p>
        <ButtonLink to="/" variant="primary">
          Go to your library
        </ButtonLink>
      </div>
    </main>
  );
}
