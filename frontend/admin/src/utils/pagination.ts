export function paginationPageWindow(
  current: number,
  totalPages: number,
): number[] {
  const count = Math.max(totalPages, 1);
  const start = Math.min(
    Math.max(current <= 4 ? 1 : current - 2, 1),
    Math.max(count - 4, 1),
  );
  return Array.from(
    { length: Math.min(5, count) },
    (_, index) => start + index,
  );
}
