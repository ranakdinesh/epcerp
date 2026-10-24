"use client";

import { startTransition, useState, useSyncExternalStore } from "react";
import { useRouter } from "next/navigation";

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const USERNAME_PATTERN = /^[a-zA-Z0-9._-]{3,64}$/;

function subscribeStatus(callback: () => void) { window.addEventListener("popstate", callback); return () => window.removeEventListener("popstate", callback); }
export default function LoginPage() {
  const passwordChanged = useSyncExternalStore(subscribeStatus, () => new URLSearchParams(window.location.search).get("session") === "password-changed", () => false);
  const router = useRouter();
  const [identifier, setIdentifier] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [rememberMe, setRememberMe] = useState(false);
  const [identifierError, setIdentifierError] = useState("");
  const [passwordError, setPasswordError] = useState("");
  const [formError, setFormError] = useState("");
  const [loading, setLoading] = useState(false);

  function validateIdentifier(value: string) {
    const normalizedValue = value.trim();

    if (!normalizedValue) {
      return "Email or username is required";
    }

    if (!EMAIL_PATTERN.test(normalizedValue) && !USERNAME_PATTERN.test(normalizedValue)) {
      return "Enter a valid email or username";
    }

    return "";
  }

  function handleForgotPassword() {
    router.push("/forgot-password");
  }

  async function handleLogin(event: React.FormEvent) {
    event.preventDefault();

    const validationError = validateIdentifier(identifier);
    if (validationError) {
      setIdentifierError(validationError);
      return;
    }

    if (!password) {
      setPasswordError("Password is required");
      return;
    }

    if (password.length < 8) {
      setPasswordError("Password must be at least 8 characters");
      return;
    }

    setIdentifierError("");
    setPasswordError("");
    setFormError("");
    setLoading(true);

    try {
      const response = await fetch("/api/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ identifier: identifier.trim(), password }),
      });

      if (!response.ok) {
        const result = (await response.json().catch(() => null)) as {
          message?: string;
        } | null;
        setFormError(result?.message ?? "Unable to sign in. Please try again.");
        return;
      }

      const result = await response.json() as {must_change_password?: boolean};
      startTransition(() => {
        router.replace(result.must_change_password ? "/change-password" : "/dashboard");
        router.refresh();
      });
    } catch {
      setFormError("Unable to reach the sign-in service. Please try again.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="min-h-screen w-full flex bg-white font-sans overflow-hidden">
      {/* Animations */}
      <style>{`
        @keyframes kenburns {
          0% { transform: scale(1) translate(0px, 0px); }
          50% { transform: scale(1.12) translate(-12px, -8px); }
          100% { transform: scale(1) translate(0px, 0px); }
        }
        @keyframes fadeUp {
          from { opacity: 0; transform: translateY(18px); }
          to { opacity: 1; transform: translateY(0); }
        }
        @keyframes floatCard {
          0%, 100% { transform: translateY(0px); }
          50% { transform: translateY(-10px); }
        }
        .kenburns {
          animation: kenburns 18s ease-in-out infinite;
        }
        .fade-up {
          animation: fadeUp 0.8s ease-out both;
        }
        .float-card {
          animation: floatCard 5s ease-in-out infinite;
        }
      `}</style>

      {/* ================= LEFT: IMAGE + ANIMATION ================= */}
      <div className="hidden lg:flex w-[52%] relative overflow-hidden">
        {/* Background Image — company hero photo (replace /login-hero.jpg to swap) */}
        <div
          className="absolute inset-0 kenburns bg-cover bg-center"
          style={{
            backgroundImage: "url('/login-hero.jpg')",
          }}
        />

        {/* Premium dark overlay — left + top anchored so the logo & text stay readable on a bright photo */}
        <div className="absolute inset-0 bg-gradient-to-r from-[#0b1b3a]/90 via-[#0b1b3a]/55 to-transparent" />
        <div className="absolute inset-0 bg-gradient-to-b from-[#0b1b3a]/75 via-transparent to-[#0b1b3a]/70" />
        <div className="absolute inset-0 bg-gradient-to-bl from-[#0b1b3a]/70 via-transparent to-transparent" />

        {/* Soft light accents */}
        <div className="absolute -top-20 -left-20 w-80 h-80 bg-blue-400/20 rounded-full blur-3xl" />
        <div className="absolute bottom-0 right-0 w-96 h-96 bg-indigo-500/20 rounded-full blur-3xl" />

        {/* Content on image */}
        <div className="relative z-10 w-full h-full flex flex-col px-12 xl:px-16 pt-8 pb-12 text-white">
          {/* Company logo — white, no white box, tucked in the top-left corner */}
          <div className="fade-up">
            <img
              src="/Vishal-Logo.svg"
              alt="Vishal Technopower"
              className="h-20 w-auto"
            />
          </div>

          {/* Center text */}
          <div className="fade-up max-w-xl mt-auto" style={{ animationDelay: "0.15s" }}>
            <h1 className="text-4xl xl:text-5xl font-extrabold leading-tight tracking-tight mb-5">
              Welcome to
              <span className="block text-transparent bg-clip-text bg-gradient-to-r from-blue-200 to-cyan-200">
                Vishal Technopower
              </span>
            </h1>

            <p className="text-blue-100/90 text-lg leading-relaxed mb-8">
              Secure ERP access for projects, workforce, approvals and
              real-time operations — built for modern engineering teams.
            </p>

            {/* Feature pills */}
            <div className="flex flex-wrap gap-3">
              {["Project Control", "Employee Hub", "Live Reports", "Secure Access"].map(
                (item) => (
                  <span
                    key={item}
                    className="px-3 py-1.5 rounded-full text-xs font-semibold bg-white/10 border border-white/15 backdrop-blur-sm"
                  >
                    {item}
                  </span>
                )
              )}
            </div>
          </div>
        </div>
      </div>

      {/* ================= RIGHT: LOGIN FORM ================= */}
      <div className="w-full lg:w-[48%] flex flex-col px-6 sm:px-10 xl:px-16 py-8 bg-white relative">
        {/* Mobile logo (small screens) */}
        <div className="lg:hidden mb-8">
            <img src="/vtpl-logo.svg" alt="Vishal Technopower" className="h-11 w-auto object-contain" />
        </div>

        <div className="flex-1 flex flex-col justify-start pt-8 lg:justify-center lg:pt-0 max-w-[440px] w-full mx-auto">
          <section className="fade-up rounded-3xl border border-slate-200/90 bg-gradient-to-b from-white to-slate-50/80 p-6 shadow-[0_24px_60px_-34px_rgba(15,23,42,0.42)] sm:p-8" aria-label="Employee login">
          <h2 className="text-center text-3xl font-extrabold text-slate-900 tracking-tight mb-7">
            LOGIN
          </h2>

          {passwordChanged && <p role="status" className="mb-4 rounded-xl border border-emerald-100 bg-emerald-50 p-3 text-sm text-emerald-800">Sign in with your updated password.</p>}
          <form onSubmit={handleLogin} className="space-y-5">
            <div>
              <label
                htmlFor="login-identifier"
                className="block text-[11px] font-bold uppercase tracking-wider text-slate-700 mb-1.5"
              >
                Email or mobile
              </label>
              <div className="relative">
                <div className="absolute inset-y-0 left-0 pl-4 flex items-center pointer-events-none" aria-hidden="true">
                  <svg className="h-5 w-5 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="2">
                    <path strokeLinecap="round" strokeLinejoin="round" d="M16 21v-2a4 4 0 00-4-4H6a4 4 0 00-4 4v2m17-11a4 4 0 11-8 0 4 4 0 018 0zM10 7a4 4 0 11-8 0 4 4 0 018 0z" />
                  </svg>
                </div>
                <input
                  id="login-identifier"
                  type="text"
                  inputMode="email"
                  autoComplete="username"
                  value={identifier}
                  onChange={(event) => {
                    setIdentifier(event.target.value);
                    setIdentifierError("");
                    setFormError("");
                  }}
                  placeholder="Enter email or username"
                  aria-invalid={Boolean(identifierError)}
                  aria-describedby={identifierError ? "login-identifier-error" : undefined}
                  className={`w-full pl-12 pr-4 py-3.5 rounded-xl bg-slate-50 border text-slate-900 placeholder-slate-400 text-sm focus:outline-none focus:ring-2 transition-all ${
                    identifierError
                      ? "border-red-500 bg-red-50 focus:border-red-600 focus:ring-red-500/20"
                      : "border-slate-200 focus:border-[#1e3a8a] focus:ring-[#1e3a8a]/20"
                  }`}
                />
              </div>
              {identifierError && (
                <p id="login-identifier-error" role="alert" className="mt-2 text-xs font-semibold text-red-600">
                  {identifierError}
                </p>
              )}
            </div>

            {/* Password */}
            <div>
              <div className="flex items-center justify-between mb-1.5">
                <label htmlFor="login-password" className="block text-[11px] font-bold uppercase tracking-wider text-slate-700">
                  Password
                </label>
                <button
                  type="button"
                  onClick={handleForgotPassword}
                  className="text-xs font-bold text-[#1e3a8a] hover:underline"
                >
                  Forgot password?
                </button>
              </div>
              <div className="relative">
                <div className="absolute inset-y-0 left-0 pl-4 flex items-center pointer-events-none">
                  <svg className="h-5 w-5 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="2">
                    <path strokeLinecap="round" strokeLinejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                  </svg>
                </div>
                <input
                  id="login-password"
                  type={showPassword ? "text" : "password"}
                  value={password}
                  onChange={(e) => {
                    setPassword(e.target.value);
                    setPasswordError("");
                    setFormError("");
                  }}
                  placeholder="Enter your password"
                  autoComplete="current-password"
                  aria-invalid={Boolean(passwordError)}
                  aria-describedby={passwordError ? "login-password-error" : undefined}
                  className={`w-full pl-12 pr-14 py-3.5 rounded-xl bg-slate-50 border text-slate-900 placeholder-slate-400 text-sm focus:outline-none focus:ring-2 transition-all ${
                    passwordError
                      ? "border-red-500 bg-red-50 focus:border-red-600 focus:ring-red-500/20"
                      : "border-slate-200 focus:border-[#1e3a8a] focus:ring-[#1e3a8a]/20"
                  }`}
                />
                <button
                  type="button"
                  onClick={() => setShowPassword((visible) => !visible)}
                  aria-label={showPassword ? "Hide password" : "Show password"}
                  aria-pressed={showPassword}
                  className="absolute right-1.5 top-1/2 -translate-y-1/2 h-11 w-11 inline-flex items-center justify-center rounded-lg text-slate-400 transition-colors hover:bg-slate-100 hover:text-[#1e3a8a] focus:outline-none focus:ring-2 focus:ring-[#1e3a8a]/20"
                >
                  {showPassword ? (
                    <svg aria-hidden="true" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="2">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M3 3l18 18M10.58 10.58a2 2 0 002.83 2.83M9.88 4.24A10.94 10.94 0 0112 4c5.52 0 9.27 4.11 10 8-.25 1.32-.9 2.64-1.85 3.75M6.61 6.61C4.93 7.76 3.69 9.66 3 12c.73 3.89 4.48 8 10 8 1.61 0 3.09-.35 4.39-.97" />
                    </svg>
                  ) : (
                    <svg aria-hidden="true" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="2">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M2.46 12C3.73 7.94 7.52 5 12 5c4.47 0 8.27 2.94 9.54 7-1.27 4.06-5.07 7-9.54 7-4.48 0-8.27-2.94-9.54-7z" />
                      <path strokeLinecap="round" strokeLinejoin="round" d="M12 15a3 3 0 100-6 3 3 0 000 6z" />
                    </svg>
                  )}
                </button>
              </div>
              {passwordError && (
                <p id="login-password-error" role="alert" className="mt-2 text-xs font-semibold text-red-600">
                  {passwordError}
                </p>
              )}
            </div>

            {/* Remember */}
            <label className="flex items-center gap-2 text-sm text-slate-600 font-medium cursor-pointer w-max">
              <input
                type="checkbox"
                checked={rememberMe}
                onChange={(e) => setRememberMe(e.target.checked)}
                className="w-4 h-4 rounded border-slate-300 text-[#1e3a8a] focus:ring-[#1e3a8a]"
              />
              Keep me signed in
            </label>

            {formError ? (
              <p
                role="alert"
                aria-live="polite"
                className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs font-semibold text-red-700"
              >
                {formError}
              </p>
            ) : null}

            {/* Button */}
            <button
              type="submit"
              disabled={loading}
              className="w-full py-4 rounded-xl bg-[#1e3a8a] hover:bg-blue-800 text-white font-bold text-sm shadow-lg shadow-blue-900/20 transition-all active:scale-[0.99] disabled:opacity-70 disabled:cursor-not-allowed flex items-center justify-center gap-2"
            >
              <span>{loading ? "Logging in..." : "Login"}</span>
            </button>
          </form>

          <p className="mt-7 text-[11px] text-slate-400 font-medium text-center">
            🔒 Secure Employee ERP • Vishal Technopower
          </p>
          </section>
        </div>
      </div>
    </div>
  );
}
