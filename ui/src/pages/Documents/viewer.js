export function clampPage(value, total) {
  return Math.min(Math.max(1, Math.trunc(Number(value)) || 1), Math.max(1, total));
}
export function escapeText(value) {
  return value.replace(/[&<>"']/g, character => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[character]);
}
export function highlightText(text, query) {
  if (!query.trim()) return escapeText(text);
  const pattern = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  return text.split(new RegExp(`(${pattern})`, 'gi')).map((part, i) => i % 2 ? `<mark>${escapeText(part)}</mark>` : escapeText(part)).join('');
}
export function pageText(content) {
  return content.items.filter(item => typeof item.str === 'string').map(item => item.str + (item.hasEOL ? '\n' : ' ')).join('').trim();
}
