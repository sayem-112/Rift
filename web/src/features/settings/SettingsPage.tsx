import { ArrowLeft, Check, ShieldCheck, SignOut } from "@phosphor-icons/react";
import { useEffect, useId, type CSSProperties, type ReactNode } from "react";
import { Link, useLocation } from "react-router";
import { Button, ButtonLink } from "../../components/Button";
import { Glass } from "../../components/glass/Glass";
import { ProgressBar } from "../../components/Progress";
import { Switch } from "../../components/Switch";
import { toast } from "../../components/toast/toastStore";
import { ACCENTS } from "../../lib/catppuccin";
import { formatBytes, formatRelative } from "../../lib/format";
import { accentStore, sampleLibraryStore } from "../../lib/prefs";
import { useStore } from "../../lib/store";
import { DEVICE_ICONS } from "../library/deviceIcon";
import { BOOK_SIZE_LIMIT, LIBRARY_QUOTA, usedBytes } from "../library/library";
import { useLibrary } from "../library/libraryStore";
import styles from "./SettingsPage.module.css";

export function SettingsPage() {
  const { books, devices, isSample } = useLibrary();
  const { hash } = useLocation();

  useEffect(() => {
    if (hash) document.getElementById(hash.slice(1))?.scrollIntoView({ block: "start" });
  }, [hash]);

  return (
    <>
      <Glass as="header" radius={26} floating refraction={52} className={styles.mobileHeader}>
        <Link to="/" className={styles.back} aria-label="Back to library">
          <ArrowLeft size={20} aria-hidden="true" />
        </Link>
        <span className={styles.mobileTitle}>Settings</span>
      </Glass>

      <main id="main" className={styles.page} tabIndex={-1}>
        <h1 className={styles.title}>Settings</h1>

        <Section title="Account">
          <Row
            label={isSample ? "Sample account" : "Signed in"}
            detail="Sign-in connects in the next build. Until then rift runs on this device only."
            action={
              <ButtonLink to="/signin" variant="secondary" size="sm" icon={<SignOut size={16} aria-hidden="true" />}>
                Sign out
              </ButtonLink>
            }
          />
        </Section>

        <Section title="Accent" description="Colors your progress, the Resume button and focus rings. Saved on this device.">
          <AccentPicker />
        </Section>

        <Section title="Storage" id="storage">
          <StorageMeter used={usedBytes(books)} />
        </Section>

        <Section title="Devices" description="Signing a device out removes its downloaded books the next time it connects.">
          {devices.map((device) => {
            const DeviceIcon = DEVICE_ICONS[device.kind];
            return (
              <Row
                key={device.id}
                icon={<DeviceIcon size={20} aria-hidden="true" />}
                label={
                  <>
                    {device.name}
                    {device.isCurrent && <span className={styles.tag}>This device</span>}
                  </>
                }
                detail={`${device.client} · ${device.isCurrent ? "active now" : `last seen ${formatRelative(device.lastSeenAt)}`}`}
                action={
                  !device.isCurrent && (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => toast({ tone: "neutral", message: `${device.name} will sign out once device sync is connected.` })}
                    >
                      Sign out
                    </Button>
                  )
                }
              />
            );
          })}
        </Section>

        <Section title="Privacy">
          <Row
            icon={<ShieldCheck size={20} aria-hidden="true" />}
            label="Encrypted and private to your account"
            detail="Your books are stored encrypted and tied to your account. We never access or share them."
          />
        </Section>

        <Section title="Development" description="Only here while the sync API is being built.">
          <SampleToggle />
        </Section>
      </main>
    </>
  );
}

function Section({ title, description, id, children }: { title: string; description?: string; id?: string; children: ReactNode }) {
  const headingId = useId();
  return (
    <section className={styles.section} aria-labelledby={headingId} id={id}>
      <div className={styles.sectionHead}>
        <h2 id={headingId} className={styles.sectionTitle}>
          {title}
        </h2>
        {description && <p className={styles.sectionDescription}>{description}</p>}
      </div>
      <div className={styles.group}>{children}</div>
    </section>
  );
}

function Row({ icon, label, detail, action }: { icon?: ReactNode; label: ReactNode; detail?: string; action?: ReactNode }) {
  return (
    <div className={styles.row}>
      {icon && <span className={styles.rowIcon}>{icon}</span>}
      <div className={styles.rowText}>
        <div className={styles.rowLabel}>{label}</div>
        {detail && <p className={styles.rowDetail}>{detail}</p>}
      </div>
      {action && <div className={styles.rowAction}>{action}</div>}
    </div>
  );
}

function AccentPicker() {
  const accent = useStore(accentStore);
  const current = ACCENTS.find((a) => a.id === accent);
  return (
    <fieldset className={styles.accents}>
      <legend className="visually-hidden">Accent color</legend>
      <div className={styles.swatches}>
        {ACCENTS.map((a) => (
          <label key={a.id} className={styles.swatch} style={{ "--swatch": a.hex } as CSSProperties} title={a.label}>
            <input
              type="radio"
              name="accent"
              value={a.id}
              checked={a.id === accent}
              onChange={() => accentStore.set(a.id)}
              className="visually-hidden"
            />
            <span className={styles.swatchDot} aria-hidden="true">
              <Check size={16} weight="bold" className={styles.swatchCheck} />
            </span>
            <span className="visually-hidden">{a.label}</span>
          </label>
        ))}
      </div>
      <p className={styles.accentName} aria-live="polite">
        {current?.label}
      </p>
    </fieldset>
  );
}

function StorageMeter({ used }: { used: number }) {
  return (
    <div className={styles.storage}>
      <p className={`${styles.storageFigure} tabular`}>
        {formatBytes(used)}
        <span className={styles.storageOf}> of {formatBytes(LIBRARY_QUOTA)}</span>
      </p>
      <ProgressBar value={used / LIBRARY_QUOTA} label="Storage used" size="md" />
      <p className={styles.rowDetail}>
        Books can be up to {formatBytes(BOOK_SIZE_LIMIT)} each. A deleted book stays in trash for 30 days before its space is freed.
      </p>
    </div>
  );
}

function SampleToggle() {
  const on = useStore(sampleLibraryStore);
  const labelId = useId();
  const detailId = useId();
  return (
    <div className={styles.row}>
      <div className={styles.rowText}>
        <div className={styles.rowLabel} id={labelId}>
          Show sample library
        </div>
        <p className={styles.rowDetail} id={detailId}>
          Placeholder books for trying the interface. Turn off to see the first-run screen.
        </p>
      </div>
      <Switch checked={on} onChange={(v) => sampleLibraryStore.set(v)} labelledBy={labelId} describedBy={detailId} />
    </div>
  );
}
