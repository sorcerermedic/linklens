import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";
import type { Api } from "./api";
import type { Link } from "./types";

const link: Link = {
  code: "abcd",
  shortUrl: "http://x/abcd",
  url: "https://example.com",
  custom: true,
  createdAt: "2026-05-01T10:00:00Z",
  expired: false,
  clicks: 1,
};

function fakeApi(overrides: Partial<Api> = {}): Api {
  return {
    listLinks: vi.fn().mockResolvedValue([link]),
    createLink: vi.fn().mockResolvedValue(link),
    getStats: vi.fn().mockResolvedValue({
      totalClicks: 1,
      byDay: [{ day: "2026-05-01", clicks: 1 }],
      byDevice: { mobile: 1 },
      byCountry: { TR: 1 },
      topReferrers: [{ referrer: "direct", clicks: 1 }],
    }),
    deleteLink: vi.fn().mockResolvedValue(undefined),
    ...overrides,
  };
}

afterEach(() => window.history.replaceState(null, "", "/"));

describe("App", () => {
  it("loads links, shows the summary and opens the stats panel", async () => {
    const api = fakeApi();
    render(<App api={api} />);
    const table = await screen.findByRole("table");
    expect(within(table).getByText("/abcd")).toBeInTheDocument();
    const summary = screen.getByRole("region", { name: "Summary" });
    expect(
      within(summary).getByText("Total clicks").parentElement,
    ).toHaveTextContent("1");

    await userEvent.click(within(table).getByRole("button", { name: "Stats" }));
    expect(api.getStats).toHaveBeenCalledWith("abcd");
    expect(
      await screen.findByRole("region", { name: "Statistics for abcd" }),
    ).toBeInTheDocument();
    expect(window.location.hash).toBe("#/stats/abcd");

    await userEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(
      screen.queryByRole("region", { name: "Statistics for abcd" }),
    ).not.toBeInTheDocument();
    expect(window.location.hash).toBe("");
  });

  it("opens the stats panel from a deep link", async () => {
    window.history.replaceState(null, "", "/#/stats/abcd");
    const api = fakeApi();
    render(<App api={api} />);
    expect(
      await screen.findByRole("region", { name: "Statistics for abcd" }),
    ).toBeInTheDocument();
    expect(api.getStats).toHaveBeenCalledWith("abcd");
  });

  it("reports a failing backend", async () => {
    render(
      <App
        api={fakeApi({
          listLinks: vi.fn().mockRejectedValue(new Error("backend down")),
        })}
      />,
    );
    expect(await screen.findByText("backend down")).toBeInTheDocument();
  });
});
