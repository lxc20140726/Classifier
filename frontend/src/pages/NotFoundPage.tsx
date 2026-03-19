export default function NotFoundPage() {
  return (
    <section className="flex min-h-[60vh] flex-col items-center justify-center gap-3 text-center">
      <p className="text-sm uppercase tracking-[0.24em] text-muted-foreground">404</p>
      <h2 className="text-4xl font-semibold tracking-tight">Page not found</h2>
      <p className="max-w-md text-sm text-muted-foreground">
        This route is not part of the current Phase 1 scaffold.
      </p>
    </section>
  )
}
