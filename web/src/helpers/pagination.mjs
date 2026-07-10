export function writePagedRows(previousRows, page, pageSize, pageRows) {
  const normalizedRows = Array.isArray(previousRows) ? [...previousRows] : [];
  const normalizedPage = Number(page) > 0 ? Number(page) : 1;
  const normalizedPageSize = Number(pageSize) > 0 ? Number(pageSize) : 1;
  const nextRows = Array.isArray(pageRows) ? pageRows : [];
  const startIndex = (normalizedPage - 1) * normalizedPageSize;
  nextRows.forEach((row, idx) => {
    normalizedRows[startIndex + idx] = row;
  });
  return normalizedRows;
}
