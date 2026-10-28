import { ArrowLeft, EnvelopeSimple, GoogleLogo, PaperPlaneTilt } from "@phosphor-icons/react";
import { useId, useRef, useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router";
import { Button } from "../../components/Button";
import { RiftMark } from "../../components/RiftMark";
import styles from "./SignInPage.module.css";

type Step = "choose" | "email" | "sent";

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

/**
 * Sign in with Google or an emailed link; there are no passwords.
 * Until the auth API lands (next build), Google continues straight to the
 * library and the email flow stops at its "check your inbox" state.
 */
export function SignInPage() {
  const navigate = useNavigate();
  const [step, setStep] = useState<Step>("choose");
  const [email, setEmail] = useState("");
  const [error, setError] = useState<string>();
  const inputRef = useRef<HTMLInputElement>(null);
  const errorId = useId();

  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (!EMAIL_PATTERN.test(email.trim())) {
      setError("Enter an email address like name@example.com.");
      inputRef.current?.focus();
      return;
    }
    setError(undefined);
    setStep("sent");
  };

  return (
    <main className={styles.page}>
      <div className={styles.column}>
        <RiftMark size={64} glow className={styles.mark} />
        <h1 className={styles.wordmark}>rift</h1>
        <p className={styles.tagline}>Your books, on every device.</p>

        <div className={styles.actions} data-step={step}>
          {step === "choose" && (
            <>
              <Button variant="primary" size="lg" icon={<GoogleLogo size={19} weight="bold" aria-hidden="true" />} onClick={() => navigate("/")}>
                Continue with Google
              </Button>
              <Button
                variant="secondary"
                size="lg"
                icon={<EnvelopeSimple size={19} aria-hidden="true" />}
                onClick={() => {
                  setStep("email");
                  requestAnimationFrame(() => inputRef.current?.focus());
                }}
              >
                Email me a sign-in link
              </Button>
            </>
          )}

          {step === "email" && (
            <form className={styles.form} onSubmit={submit} noValidate>
              <label htmlFor="email" className={styles.label}>
                Email
              </label>
              <input
                ref={inputRef}
                id="email"
                type="email"
                inputMode="email"
                autoComplete="email"
                placeholder="name@example.com"
                className={styles.input}
                value={email}
                aria-invalid={error ? true : undefined}
                aria-describedby={error ? errorId : undefined}
                onChange={(e) => {
                  setEmail(e.target.value);
                  if (error) setError(undefined);
                }}
              />
              {error && (
                <p id={errorId} className={styles.error} role="alert">
                  {error}
                </p>
              )}
              <Button type="submit" variant="primary" size="lg" icon={<PaperPlaneTilt size={18} weight="fill" aria-hidden="true" />}>
                Send link
              </Button>
              <Button variant="ghost" size="md" icon={<ArrowLeft size={16} aria-hidden="true" />} onClick={() => setStep("choose")}>
                Other ways to sign in
              </Button>
            </form>
          )}

          {step === "sent" && (
            <div className={styles.sent} role="status">
              <span className={styles.sentIcon}>
                <EnvelopeSimple size={24} weight="fill" aria-hidden="true" />
              </span>
              <h2 className={styles.sentTitle}>Check your inbox</h2>
              <p className={styles.sentBody}>
                We'll send a sign-in link to <strong>{email.trim()}</strong>. Open it on this device to finish signing in.
              </p>
              <p className={styles.devNote}>
                Email sign-in connects in the next build. <Link to="/">Open the library</Link>
              </p>
              <Button variant="ghost" size="md" onClick={() => setStep("email")}>
                Use a different email
              </Button>
            </div>
          )}
        </div>

        <p className={styles.privacy}>No passwords. Your books are encrypted and private to your account.</p>
      </div>
    </main>
  );
}
