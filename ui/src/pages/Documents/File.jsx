import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useHistory, useLocation } from 'react-router-dom';
import { Button, Dropdown, Spinner } from 'react-bootstrap';
import { BsArrowsFullscreen, BsChevronLeft, BsChevronRight, BsDownload, BsLayoutSidebar, BsSearch, BsArrowClockwise, BsX } from 'react-icons/bs';
import { Document, Page } from 'react-pdf';
import 'react-pdf/dist/Page/TextLayer.css';
import 'react-pdf/dist/Page/AnnotationLayer.css';
import { toast } from 'react-toastify';
import api from '../../services/api.service';
import constants from '../../common/constants';
import { clampPage, highlightText, pageText } from './viewer';
import styles from './Viewer.module.scss';

function PageThumbnail({ number, selected, onSelect }) {
  const ref = useRef(null);
  const [visible, setVisible] = useState(false);
  useEffect(() => {
    const observer = new IntersectionObserver(([entry]) => setVisible(entry.isIntersecting), { rootMargin: '180px' });
    observer.observe(ref.current);
    return () => observer.disconnect();
  }, []);
  useEffect(() => { if (selected) ref.current?.scrollIntoView({ block: 'nearest', inline: 'nearest' }); }, [selected]);
  return <button ref={ref} className={styles.thumb} aria-label={`Go to page ${number}`} aria-current={selected ? 'page' : undefined} onClick={() => onSelect(number)}><div>{visible && <Page pageNumber={number} width={112} devicePixelRatio={1} renderTextLayer={false} renderAnnotationLayer={false} loading="…" error="Preview unavailable" />}</div><span>Page {number}</span></button>;
}

export default function FileViewer({ file, sourceUrl, historical = false }) {
  const location = useLocation();
  const history = useHistory();
  const [pdf, setPdf] = useState(null);
  const [error, setError] = useState(false);
  const [retry, setRetry] = useState(0);
  const [showPages, setShowPages] = useState(() => window.innerWidth > 760);
  const [zoom, setZoom] = useState('page');
  const [rotation, setRotation] = useState(0);
  const [size, setSize] = useState({ width: 800, height: 650 });
  const [aspect, setAspect] = useState(0.75);
  const [showFind, setShowFind] = useState(false);
  const [query, setQuery] = useState('');
  const [matches, setMatches] = useState([]);
  const [searching, setSearching] = useState(false);
  const [searchError, setSearchError] = useState('');
  const [pageInput, setPageInput] = useState('1');
  const viewer = useRef(null);
  const stage = useRef(null);
  const textCache = useRef(new Map());
  const findRef = useRef(null);
  const total = pdf?.numPages || 1;
  const page = clampPage(new URLSearchParams(location.search).get('page'), total);
  const url = sourceUrl || `${constants.ROOT_URL}/documents/${encodeURIComponent(file.id)}`;
  const pdfSource = useMemo(() => ({ url, withCredentials: true }), [url]);
  const onLoad = useCallback(document => { setPdf(document); setError(false); textCache.current.clear(); }, []);
  const textRenderer = useCallback(({ str }) => highlightText(str, query), [query]);

  function goTo(value) {
    const next = clampPage(value, total);
    const params = new URLSearchParams(location.search);
    params.set('page', String(next));
    history.replace({ ...location, search: params.toString() });
    setPageInput(String(next));
    stage.current?.scrollTo({ top: 0, left: 0 });
  }
  useEffect(() => setPageInput(String(page)), [page]);
  useEffect(() => {
    if (!pdf) return;
    let active = true;
    pdf.getPage(page).then(p => { if (active) { const viewport = p.getViewport({ scale: 1, rotation }); setAspect(viewport.width / viewport.height); } }).catch(() => {});
    return () => { active = false; };
  }, [pdf, page, rotation]);
  useEffect(() => {
    if (!stage.current) return;
    const observer = new ResizeObserver(([entry]) => setSize({ width: entry.contentRect.width, height: entry.contentRect.height }));
    observer.observe(stage.current);
    return () => observer.disconnect();
  }, [pdf, showPages]);
  useEffect(() => {
    if (showFind) findRef.current?.focus();
  }, [showFind]);
  useEffect(() => {
    let active = true;
    const term = query.trim().toLocaleLowerCase();
    setMatches([]); setSearchError('');
    if (!pdf || !term) { setSearching(false); return; }
    setSearching(true);
    const timer = setTimeout(async () => {
      const found = [];
      try {
        for (let number = 1; number <= pdf.numPages && active; number++) {
          let text = textCache.current.get(number);
          if (text === undefined) {
            const p = await pdf.getPage(number);
            text = pageText(await p.getTextContent());
            if (!active) return;
            textCache.current.set(number, text);
          }
          if (text.toLocaleLowerCase().includes(term)) found.push(number);
        }
        if (active) setMatches(found);
      } catch { if (active) setSearchError('Some page text could not be read. Try again.'); }
      finally { if (active) setSearching(false); }
    }, 250);
    return () => { active = false; clearTimeout(timer); };
  }, [query, pdf]);

  async function download(type) {
    try {
      let blob;
      if (sourceUrl) {
        const response = await fetch(sourceUrl);
        if (!response.ok) throw new Error('Could not download this preview.');
        blob = await response.blob();
      } else blob = await api.download(file.id, type);
      const href = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = href; link.download = `${file.data.name}.${type}`;
      document.body.appendChild(link); link.click(); link.remove();
      setTimeout(() => URL.revokeObjectURL(href), 60000);
    } catch (err) { toast.error(err.message || 'Download failed'); }
  }
  const availableWidth = Math.max(100, size.width - 56);
  const width = zoom === 'page' ? Math.min(availableWidth, Math.max(100, size.height - 56) * aspect) : zoom === 'width' ? availableWidth : 800 * Number(zoom) / 100;
  function keyboard(event) {
    if (event.target.closest('input, textarea, select, [contenteditable=true]')) return;
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'f') { event.preventDefault(); event.stopPropagation(); setShowFind(true); return; }
    if (['ArrowRight', 'PageDown', 'ArrowLeft', 'PageUp', 'Home', 'End'].includes(event.key)) {
      event.preventDefault(); event.stopPropagation();
      goTo(event.key === 'Home' ? 1 : event.key === 'End' ? total : page + (['ArrowRight', 'PageDown'].includes(event.key) ? 1 : -1));
    }
  }
  return <section ref={viewer} className={styles.viewer} aria-label="Document viewer" tabIndex={0} onKeyDown={keyboard}>
    <div className={styles.toolbar}>
      <button aria-label="Toggle page thumbnails" aria-pressed={showPages} onClick={() => setShowPages(value => !value)}><BsLayoutSidebar /></button>
      <button aria-label="Previous page" disabled={!pdf || page <= 1} onClick={() => goTo(page - 1)}><BsChevronLeft /></button>
      <form onSubmit={event => { event.preventDefault(); goTo(pageInput); }}><label>Page <input aria-label="Page number" type="number" min="1" max={total} value={pageInput} disabled={!pdf} onChange={event => setPageInput(event.target.value)} onBlur={() => goTo(pageInput)} /> of {pdf ? total : '…'}</label></form>
      <button aria-label="Next page" disabled={!pdf || page >= total} onClick={() => goTo(page + 1)}><BsChevronRight /></button>
      <span className={styles.spacer} />
      <select aria-label="Page zoom" value={zoom} onChange={event => setZoom(event.target.value)}><option value="page">Fit page</option><option value="width">Fit width</option>{[50,75,100,125,150,200].map(value => <option key={value} value={String(value)}>{value}%</option>)}</select>
      <button aria-label="Rotate page" onClick={() => setRotation(value => (value + 90) % 360)}><BsArrowClockwise /></button>
      <button aria-label="Find text in document" aria-pressed={showFind} onClick={() => setShowFind(value => !value)}><BsSearch /></button>
      <button aria-label="Full screen" onClick={async () => { try { if (document.fullscreenElement) await document.exitFullscreen(); else await viewer.current.requestFullscreen(); } catch { toast.info('Full screen is not available in this browser.'); } }}><BsArrowsFullscreen /></button>
      <Dropdown align="end"><Dropdown.Toggle size="sm" variant="outline-secondary" aria-label="Download document"><BsDownload /></Dropdown.Toggle><Dropdown.Menu><Dropdown.Item onClick={() => download('pdf')}>Download PDF</Dropdown.Item>{!historical && <Dropdown.Item onClick={() => download('rmdoc')}>Download original .rmdoc</Dropdown.Item>}</Dropdown.Menu></Dropdown>
    </div>
    {showFind && <div className={styles.find}><input ref={findRef} aria-label="Find text" placeholder="Find printed text in this document…" value={query} onChange={event => setQuery(event.target.value)} /><span role="status">{searching ? 'Searching pages…' : query.trim() ? `${matches.length} matching pages` : ''}</span><button disabled={!matches.length} aria-label="Previous matching page" onClick={() => goTo([...matches].reverse().find(number => number < page) || matches.at(-1))}><BsChevronLeft /></button><button disabled={!matches.length} aria-label="Next matching page" onClick={() => goTo(matches.find(number => number > page) || matches[0])}><BsChevronRight /></button><button aria-label="Close document search" onClick={() => { setShowFind(false); setQuery(''); }}><BsX /></button>{searchError && <p role="alert">{searchError}</p>}<p>Find searches embedded PDF text. Handwriting requires a searchable OCR index.</p></div>}
    <Document key={`${url}:${retry}`} className={styles.document} file={pdfSource} onLoadSuccess={onLoad} onLoadError={() => { setError(true); setPdf(null); }} loading={<div className={styles.message} role="status"><Spinner animation="border" /><p>Preparing your document…</p></div>} error={<div className={styles.message} role="alert"><h2>This document could not be rendered</h2><p>The server may not support this notebook format yet, or the connection was interrupted.</p><Button onClick={() => { setError(false); setRetry(value => value + 1); }}>Try again</Button>{!historical && <Button variant="outline-secondary" onClick={() => download('rmdoc')}>Download original</Button>}</div>}>
      {pdf && !error && <div className={styles.body}>
        {showPages && <nav className={styles.sidebar} aria-label="Notebook pages">{Array.from({ length: total }, (_, index) => <PageThumbnail key={index + 1} number={index + 1} selected={page === index + 1} onSelect={goTo} />)}</nav>}
        <div ref={stage} className={styles.stage}><div className={styles.pageArea}><div className={styles.paper} style={{ width }}><Page pageNumber={page} width={width} rotate={rotation} renderTextLayer renderAnnotationLayer customTextRenderer={textRenderer} externalLinkTarget="_blank" externalLinkRel="noopener noreferrer" loading={<div className={styles.message}>Rendering page {page}…</div>} error={<div className={styles.message} role="alert">This page could not be rendered. Try another page or reload the document.</div>} /></div></div></div>
      </div>}
    </Document>
    <div className={styles.status}><span>{historical ? 'Historical snapshot' : 'Read-only preview'} · {file.data.name}</span><span>← / → to turn pages · Ctrl/⌘ F to find text</span></div>
  </section>;
}
