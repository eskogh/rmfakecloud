import test from "node:test";
import assert from "node:assert/strict";
import api from "./api.service.js";

for (const [name, request] of [
  ["list", () => api.listDocument()],
  ["create", () => api.createFolder({ name: "Notes", parentId: "" })],
  ["move", () => api.moveDocument("doc", "Notes", "folder")],
  ["delete", () => api.deleteDocument("doc")],
  ["download", () => api.download("doc", "rmdoc")],
]) {
  test(`${name} rejects JSON errors instead of reporting success`, async (context) => {
    context.mock.method(
      globalThis,
      "fetch",
      async () =>
        new Response(JSON.stringify({ error: "Request failed on server" }), {
          status: 409,
          headers: { "Content-Type": "application/json" },
        }),
    );
    await assert.rejects(request(), /Request failed on server/);
  });
}

test("failed download with no content type rejects and cannot become a downloaded error blob", async (context) => {
  context.mock.method(
    globalThis,
    "fetch",
    async () => new Response(null, { status: 503 }),
  );
  await assert.rejects(api.download("doc"), /503/);
});

test("move preserves the document name and represents the root with an empty parent", async (context) => {
  let body;
  context.mock.method(globalThis, "fetch", async (url, options) => {
    assert.equal(url, "/ui/api/documents");
    assert.equal(options.method, "PUT");
    body = JSON.parse(options.body);
    return new Response(null, { status: 200 });
  });
  await api.moveDocument("doc", "Field notes", "");
  assert.deepEqual(body, {
    documentId: "doc",
    name: "Field notes",
    parentId: "",
  });
});
