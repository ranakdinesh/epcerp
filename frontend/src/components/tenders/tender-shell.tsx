"use client";

import Link from "next/link";
import { useState, type ReactNode } from "react";
import { useRouter } from "next/navigation";
import {
  Bars3Icon,
  ChevronDownIcon,
  ChevronRightIcon,
  XMarkIcon,
} from "@heroicons/react/24/outline";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import { tenderWorkflowReference } from "./tender-workflow-reference";
import "./tender-shell.css";

export function TenderShell({
  children,
  action,
}: {
  children: ReactNode;
  action: ReactNode;
}) {
  const user = useAuthenticatedUser();
  const router = useRouter();
  const [railOpen, setRailOpen] = useState(false);
  const [stepCode, setStepCode] = useState<string | null>(null);
  const [logoutError, setLogoutError] = useState("");
  const step = tenderWorkflowReference.find(
    (item) => item.step_code === stepCode,
  );
  const tracks = [
    ...new Set(tenderWorkflowReference.map((item) => item.track)),
  ];
  async function logout() {
    const response = await fetch("/api/auth/logout", { method: "POST" });
    if (response.ok) {
      router.replace("/login");
      router.refresh();
    } else setLogoutError("Unable to sign out. Please retry.");
  }
  return (
    <div className={`tender-shell ${railOpen ? "tender-rail-open" : ""}`}>
      <header className="tender-appbar">
        <Link href="/dashboard" className="tender-brand">
          VTPL <span>CORE</span>
        </Link>
        <nav aria-label="Breadcrumb" className="tender-breadcrumb">
          <Link href="/dashboard">Workspace</Link>
          <ChevronRightIcon aria-hidden="true" />
          <Link href="/tenders" aria-current="page">
            Tender FMS
          </Link>
        </nav>
        <details className="tender-account">
          <summary>
            <span className="tender-avatar">
              {user.username.slice(0, 1).toUpperCase()}
            </span>
            <span>{user.username}</span>
            <ChevronDownIcon aria-hidden="true" />
          </summary>
          <div className="tender-account-menu">
            <button onClick={() => void logout()}>Sign out</button>
            {logoutError ? <p role="alert">{logoutError}</p> : null}
          </div>
        </details>
      </header>
      <aside className="tender-rail" aria-label="Tender workflow">
        <div className="tender-rail-heading">
          Tender workflow
          <button
            aria-label="Close workflow navigation"
            onClick={() => setRailOpen(false)}
          >
            <XMarkIcon />
          </button>
        </div>
        <button
          className={`tender-rail-item ${!step ? "is-active" : ""}`}
          aria-current={!step ? "page" : undefined}
          onClick={() => {
            setStepCode(null);
            setRailOpen(false);
          }}
        >
          Tender register
        </button>
        <p className="tender-reference-label">26 steps · Reference workflow</p>
        {tracks.map((track) => (
          <div key={track}>
            <h2 className="tender-track">{track}</h2>
            {tenderWorkflowReference
              .filter((item) => item.track === track)
              .map((item) => (
                <button
                  key={item.step_code}
                  className={`tender-rail-item ${stepCode === item.step_code ? "is-active" : ""}`}
                  aria-current={
                    stepCode === item.step_code ? "step" : undefined
                  }
                  onClick={() => {
                    setStepCode(item.step_code);
                    setRailOpen(false);
                  }}
                >
                  <span className="tender-step-number">
                    {item.global_order}
                  </span>
                  <span>{item.task_title}</span>
                </button>
              ))}
          </div>
        ))}
      </aside>
      <main className="tender-main">
        <header className="tender-page-heading">
          <button
            className="tender-menu"
            aria-label="Open workflow navigation"
            aria-expanded={railOpen}
            onClick={() => setRailOpen((value) => !value)}
          >
            <Bars3Icon />
          </button>
          <h1>{step ? step.task_title : "Tender register"}</h1>
          {!step ? (
            action
          ) : (
            <button
              className="tender-secondary"
              onClick={() => setStepCode(null)}
            >
              Back to register
            </button>
          )}
        </header>
        <div className="tender-content" hidden={Boolean(step)}>
          {children}
        </div>
        {step ? (
          <div className="tender-content">
            <section className="tender-step-card">
              <div className="tender-step-meta">
                <span>
                  {step.step_code} · Step {step.global_order} of 26
                </span>
                <span className="tender-reference-badge">Reference</span>
              </div>
              <h2>{step.track}</h2>
              <p>{step.task_detail}</p>
              <dl>
                <div>
                  <dt>Responsible role</dt>
                  <dd>{step.role}</dd>
                </div>
                <div>
                  <dt>Document required</dt>
                  <dd>{step.needs_document ? "Yes" : "No"}</dd>
                </div>
                <div>
                  <dt>Approval required</dt>
                  <dd>{step.is_approval ? "Yes" : "No"}</dd>
                </div>
              </dl>
              <div className="tender-step-availability">
                <strong>
                  {step.step_code === "TS-01"
                    ? "Receive and register a tender"
                    : "Workflow actions not yet available"}
                </strong>
                <p>
                  {step.step_code === "TS-01"
                    ? "Upload the tender PDF from the register, then open its extracted details for review."
                    : "This step comes from the uploaded Tender FMS. Assignments, documents, approvals and completion are not yet connected in this app."}
                </p>
                <button
                  className="tender-primary"
                  onClick={() => setStepCode(null)}
                >
                  Open tender register
                </button>
              </div>
            </section>
          </div>
        ) : null}
      </main>
    </div>
  );
}
