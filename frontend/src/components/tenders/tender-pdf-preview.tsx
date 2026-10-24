"use client";
import { useEffect, useState } from "react";
import Image from "next/image";

export function TenderPdfPreview({
  id,
  page,
  onClose,
}: {
  id: string;
  page: number;
  onClose: () => void;
}) {
  const [url, setUrl] = useState("");
  const [error, setError] = useState("");
  useEffect(() => {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 45000);
    let active = true;
    let objectUrl = "";
    void fetch(`/api/tenders/${id}/source/page/${page}`, {
      signal: controller.signal,
      cache: "no-store",
    })
      .then(async (response) => {
        if (!response.ok)
          throw new Error(
            "Unable to load original PDF. Try the download link or sign in again.",
          );
        if (!response.headers.get("content-type")?.startsWith("image/png"))
          throw new Error("The original PDF could not be displayed.");
        return response.blob();
      })
      .then((blob) => {
        if (active) {
          objectUrl = URL.createObjectURL(blob);
          setUrl(objectUrl);
        }
      })
      .catch((error) => {
        if (active)
          setError(
            error.name === "AbortError"
              ? "PDF preview took too long to load. Use the download link to check this page."
              : error.message,
          );
      })
      .finally(() => clearTimeout(timeout));
    return () => {
      active = false;
      controller.abort();
      clearTimeout(timeout);
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [id, page]);
  return (
    <aside
      className="min-w-0 rounded-lg border border-blue-200 bg-slate-50 p-3"
      aria-label="Original PDF comparison"
    >
      <div className="flex items-center justify-between gap-2">
        <h4 className="font-bold">Original PDF · page {page}</h4>
        <button
          type="button"
          onClick={onClose}
          className="min-h-11 px-2 text-sm text-blue-700"
        >
          Close preview
        </button>
      </div>
      <p className="mb-2 text-xs text-slate-600">
        Image rendered directly from this original PDF page.
      </p>
      {error ? (
        <p role="alert" className="text-sm text-red-800">
          {error}
        </p>
      ) : !url ? (
        <p role="status">Loading original PDF…</p>
      ) : (
        <div className="h-[28rem] overflow-auto rounded border bg-white">
          <Image
            unoptimized
            width={1200}
            height={1600}
            alt={`Original tender PDF page ${page}`}
            src={url}
            className="h-auto w-full"
          />
        </div>
      )}
      <a
        href={`/api/tenders/${id}/source/pdf`}
        className="mt-2 block min-h-11 py-3 text-sm font-semibold text-blue-700"
      >
        Download PDF if preview is unavailable
      </a>
    </aside>
  );
}
