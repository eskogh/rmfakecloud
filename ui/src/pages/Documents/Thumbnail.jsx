import { useEffect, useRef, useState } from "react";
import { Document, Page } from "react-pdf";
import constants from "../../common/constants";
import FileIcon from "./FileIcon";
import styles from "./Browser.module.scss";

export default function Thumbnail({ node }) {
  const container = useRef(null);
  const [visible, setVisible] = useState(false);
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    if (node.isLeaf) {
      const observer = new IntersectionObserver(
        ([entry]) => setVisible(entry.isIntersecting),
        { rootMargin: "100px" },
      );
      observer.observe(container.current);
      return () => observer.disconnect();
    }
  }, [node.isLeaf]);
  const fallback = (
    <div className={styles.coverIcon}>
      <FileIcon file={node.data} />
      <span>
        {node.isLeaf
          ? node.data.type === "TODO"
            ? "Document"
            : node.data.type || "Document"
          : `${node.children.length} ${node.children.length === 1 ? "item" : "items"}`}
      </span>
    </div>
  );
  return (
    <div
      ref={container}
      className={`${styles.cover} ${!node.isLeaf ? styles.folderCover : ""}`}
      aria-hidden="true"
    >
      {visible && node.isLeaf && !failed ? (
        <Document
          file={`${constants.ROOT_URL}/documents/${encodeURIComponent(node.id)}`}
          loading={fallback}
          error={fallback}
          onLoadError={() => setFailed(true)}
        >
          <Page
            pageNumber={1}
            width={180}
            devicePixelRatio={1}
            renderTextLayer={false}
            renderAnnotationLayer={false}
            loading={fallback}
            onRenderError={() => setFailed(true)}
          />
        </Document>
      ) : (
        fallback
      )}
    </div>
  );
}
