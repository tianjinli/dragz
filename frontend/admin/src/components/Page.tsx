import type { PropsWithChildren, ReactNode } from "react";

export function Page({
  children,
  description,
  extra,
}: PropsWithChildren<{
  description?: ReactNode;
  extra?: ReactNode;
  title?: ReactNode;
}>) {
  return (
    <section className="page-shell">
      {(description || extra) && (
        <header className="page-heading">
          <div>
            {description && (
              <div className="page-description">{description}</div>
            )}
          </div>
          {extra}
        </header>
      )}
      {children}
    </section>
  );
}
