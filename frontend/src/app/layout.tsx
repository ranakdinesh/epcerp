import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "VTPL CORE | epcERP",
  description:
    "The independent epcERP project execution workspace.",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html suppressHydrationWarning lang="en" data-scroll-behavior="smooth" className="h-full antialiased">
      <body className="min-h-full flex flex-col"><script dangerouslySetInnerHTML={{ __html: `(function(){var t;try{t=localStorage.getItem('epcerp-theme')}catch(e){}document.documentElement.dataset.theme=t==='light'||t==='dark'?t:window.matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light'})()` }} />{children}</body>
    </html>
  );
}
