import { useCallback, useEffect, useState } from "react";
import api from "../services/api.service";
export default function useResource(path) {
  const [state, setState] = useState({ data: null, error: "", loading: true });
  const [revision, setRevision] = useState(0);
  const refresh = useCallback(() => setRevision((n) => n + 1), []);
  useEffect(() => {
    const controller = new AbortController();
    setState((s) => ({ ...s, loading: true, error: "" }));
    api
      .resource(path, controller.signal)
      .then((data) => setState({ data, error: "", loading: false }))
      .catch((error) => {
        if (error.name !== "AbortError")
          setState((s) => ({ ...s, error: error.message, loading: false }));
      });
    return () => controller.abort();
  }, [path, revision]);
  return { ...state, refresh };
}
