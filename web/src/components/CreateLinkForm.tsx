import { useState, type FormEvent } from "react";
import type { CreateLinkInput, Link } from "../types";
import { CheckIcon, CopyIcon, LinkIcon } from "./icons";

interface Props {
  onCreate: (input: CreateLinkInput) => Promise<Link>;
}

export function CreateLinkForm({ onCreate }: Props) {
  const [url, setUrl] = useState("");
  const [alias, setAlias] = useState("");
  const [expiresAt, setExpiresAt] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [created, setCreated] = useState<Link | null>(null);
  const [busy, setBusy] = useState(false);
  const [copied, setCopied] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setCreated(null);
    setCopied(false);
    if (!url.trim()) {
      setError("Please enter a URL.");
      return;
    }
    const input: CreateLinkInput = { url: url.trim() };
    if (alias.trim()) input.alias = alias.trim();
    if (expiresAt) input.expiresAt = new Date(expiresAt).toISOString();

    setBusy(true);
    try {
      const link = await onCreate(input);
      setCreated(link);
      setUrl("");
      setAlias("");
      setExpiresAt("");
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  async function handleCopy() {
    if (!created) return;
    try {
      await navigator.clipboard.writeText(created.shortUrl);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      setCopied(false);
    }
  }

  return (
    <form
      className="card create-form"
      onSubmit={handleSubmit}
      aria-label="Create short link"
      noValidate
    >
      <div className="card-head">
        <div>
          <h2 className="card-title">Shorten a URL</h2>
          <p className="card-desc">
            Generated codes are Base62, or bring your own alias.
          </p>
        </div>
        <span className="card-icon">
          <LinkIcon />
        </span>
      </div>

      <div className="field">
        <label htmlFor="cl-url">Destination URL</label>
        <input
          id="cl-url"
          type="url"
          inputMode="url"
          placeholder="https://example.com/a/very/long/path"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          aria-invalid={error ? true : undefined}
        />
      </div>
      <div className="field-row">
        <div className="field">
          <label htmlFor="cl-alias">
            Custom alias <span className="optional">optional</span>
          </label>
          <div className="input-prefix">
            <span aria-hidden="true">/</span>
            <input
              id="cl-alias"
              type="text"
              placeholder="kampanya"
              value={alias}
              onChange={(e) => setAlias(e.target.value)}
            />
          </div>
        </div>
        <div className="field">
          <label htmlFor="cl-exp">
            Expires at <span className="optional">optional</span>
          </label>
          <input
            id="cl-exp"
            type="datetime-local"
            value={expiresAt}
            onChange={(e) => setExpiresAt(e.target.value)}
          />
        </div>
      </div>

      <button
        type="submit"
        className="btn btn-primary btn-block"
        disabled={busy}
      >
        {busy ? "Creating…" : "Create short link"}
      </button>

      {error && (
        <p role="alert" className="form-error">
          {error}
        </p>
      )}
      {created && (
        <div className="result" role="status">
          <span className="result-label">Ready to share</span>
          <div className="result-row">
            <a
              href={created.shortUrl}
              target="_blank"
              rel="noreferrer"
              className="mono"
            >
              {created.shortUrl}
            </a>
            <button
              type="button"
              className="btn btn-ghost btn-icon"
              onClick={() => void handleCopy()}
              aria-label="Copy short link"
            >
              {copied ? <CheckIcon /> : <CopyIcon />}
            </button>
          </div>
        </div>
      )}
    </form>
  );
}
