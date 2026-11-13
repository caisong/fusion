// The backend may mount the app under a subpath (FUSION_BASE_PATH). It exposes
// the mount point through a meta tag in the served index.html because the same
// prebuilt bundle has to work at any path. Missing tag means the domain root,
// which is also the case for the Vite dev server.
function readInjectedBasePath(): string {
  if (typeof document === "undefined") {
    return "";
  }

  return (
    document.querySelector<HTMLMetaElement>('meta[name="fusion-base-path"]')
      ?.content ?? ""
  );
}

export function normalizeBasePath(raw: string): string {
  const trimmed = raw.trim();
  if (trimmed === "" || trimmed === "/") {
    return "";
  }

  return `/${trimmed.replace(/^\/+|\/+$/g, "")}`;
}

// Empty at the domain root; otherwise a prefix such as "/fusion".
export const BASE_PATH = normalizeBasePath(readInjectedBasePath());

export function withBasePath(path: string): string {
  return `${BASE_PATH}${path.startsWith("/") ? path : `/${path}`}`;
}
