import { Info, WarningCircle, X } from "@phosphor-icons/react";
import { useNavigate } from "react-router";
import { useStore } from "../../lib/store";
import { Glass } from "../glass/Glass";
import { dismissToast, holdToast, releaseToast, toastStore } from "./toastStore";
import styles from "./Toaster.module.css";

export function Toaster() {
  const toasts = useStore(toastStore);
  const navigate = useNavigate();

  return (
    <section className={styles.region} aria-label="Notifications">
      <ol className={styles.list} role="status" aria-live="polite">
        {toasts.map((t) => (
          <li key={t.id}>
            <Glass
              floating
              radius={18}
              refraction={0}
              className={[styles.toast, t.tone === "danger" && styles.danger].filter(Boolean).join(" ")}
              onPointerEnter={() => holdToast(t.id)}
              onPointerLeave={() => releaseToast(t.id)}
              onFocus={() => holdToast(t.id)}
              onBlur={() => releaseToast(t.id)}
            >
              {t.tone === "danger" ? (
                <WarningCircle className={styles.icon} size={20} weight="fill" aria-hidden="true" />
              ) : (
                <Info className={styles.icon} size={20} weight="fill" aria-hidden="true" />
              )}
              <p className={styles.message}>{t.message}</p>
              {t.action && (
                <button
                  type="button"
                  className={styles.action}
                  onClick={() => {
                    t.action?.onClick?.();
                    if (t.action?.href) navigate(t.action.href);
                    dismissToast(t.id);
                  }}
                >
                  {t.action.label}
                </button>
              )}
              <button type="button" className={styles.close} onClick={() => dismissToast(t.id)} aria-label="Dismiss">
                <X size={14} weight="bold" aria-hidden="true" />
              </button>
            </Glass>
          </li>
        ))}
      </ol>
    </section>
  );
}
