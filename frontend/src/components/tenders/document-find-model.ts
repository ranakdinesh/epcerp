export type DocumentPage = { number: number | null; text: string };
export type PageMatch = DocumentPage & { excerpt: string };

// Only generated page headings are boundaries; table headings belong to their page.
export function documentPages(markdown: string): DocumentPage[] {
  const body = markdown.split(/^## Canonical page manifest\s*$/m)[0];
  const headings = [...body.matchAll(/^## Page (\d+)\s*$/gm)];
  if (!headings.length) return [{ number: null, text: body.trim() }];
  return headings.map((heading, index) => ({
    number: Number(heading[1]),
    text: body
      .slice(
        heading.index! + heading[0].length,
        headings[index + 1]?.index ?? body.length,
      )
      .trim(),
  }));
}

export function searchTerms(query: string): string[] {
  return [
    ...new Set(
      query
        .split("|")
        .map((term) => term.trim().replace(/\s+/g, " ").toLowerCase())
        .filter(Boolean),
    ),
  ];
}

export function findPages(pages: DocumentPage[], query: string): PageMatch[] {
  const terms = searchTerms(query);
  if (!terms.length) return [];
  return pages
    .flatMap((page) => {
      const text = page.text.replace(/\s+/g, " ");
      const lower = text.toLowerCase();
      const hits = terms
        .map((term) => ({ term, position: lower.indexOf(term) }))
        .filter((hit) => hit.position >= 0)
        .sort((a, b) => b.term.length - a.term.length);
      if (!hits.length) return [];
      const position = hits[0].position;
      const score = hits.reduce((total, hit) => total + hit.term.length, 0);
      const start = Math.max(0, position - 100);
      const end = Math.min(text.length, position + 360);
      return [
        {
          ...page,
          score,
          excerpt: `${start ? "…" : ""}${text.slice(start, end)}${end < text.length ? "…" : ""}`,
        },
      ];
    })
    .sort((a, b) => b.score - a.score);
}
