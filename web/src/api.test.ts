import { afterEach, describe, expect, it, vi } from "vitest";
import { api, ApiError } from "./api";
function mockFetch(status: number, body: unknown) {
  const fn = vi.fn().mockResolvedValue(
    new Response(body === undefined ? null : JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    }),
  );
  vi.stubGlobal("fetch", fn);
  return fn;
}
afterEach(() => vi.unstubAllGlobals());
describe("api client", () => {
  it("posts JSON when creating a link", async () => {
    const fetchMock = mockFetch(201, { code: "abcd" });
    const res = await api.createLink({ url: "https://example.com" });
    expect(res.code).toBe("abcd");
    const [path, init] = fetchMock.mock.calls[0];
    expect(path).toBe("/api/links");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toEqual({ url: "https://example.com" });
  });
  it("throws ApiError with the server message", async () => {
    mockFetch(409, { error: "alias already taken" });
    await expect(
      api.createLink({ url: "https://example.com", alias: "x" }),
    ).rejects.toEqual(new ApiError(409, "alias already taken"));
  });
  it("handles 204 on delete and encodes the code", async () => {
    const fetchMock = mockFetch(204, undefined);
    await expect(api.deleteLink("a b")).resolves.toBeUndefined();
    expect(fetchMock.mock.calls[0][0]).toBe("/api/links/a%20b");
  });
});
