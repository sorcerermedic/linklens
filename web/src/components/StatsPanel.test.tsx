import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { StatsPanel } from "./StatsPanel";
import { toSortedEntries } from "../lib/entries";
import type { Link, Stats } from "../types";

const stats: Stats = {
  totalClicks: 5,
  byDay: [
    { day: "2026-05-01", clicks: 2 },
    { day: "2026-05-02", clicks: 3 },
  ],
  byDevice: { desktop: 3, mobile: 2 },
  byCountry: { TR: 3, US: 2 },
  topReferrers: [
    { referrer: "github.com", clicks: 4 },
    { referrer: "direct", clicks: 1 },
  ],
};

const link: Link = {
  code: "abcd",
  shortUrl: "http://x/abcd",
  url: "https://kahvedukkani.com.tr/kampanya",
  custom: true,
  createdAt: "2026-05-01T10:00:00Z",
  expired: false,
  clicks: 5,
};

describe("StatsPanel", () => {
  it("shows totals, peak day, countries and referrers", () => {
    render(
      <StatsPanel code="abcd" stats={stats} link={link} onClose={vi.fn()} />,
    );
    expect(screen.getByTestId("total-clicks")).toHaveTextContent("5");
    expect(screen.getByText("Mobile share").parentElement).toHaveTextContent(
      "40%",
    );
    expect(
      screen.getByText("kahvedukkani.com.tr/kampanya"),
    ).toBeInTheDocument();

    const countries = within(
      screen.getByRole("list", { name: "Countries" }),
    ).getAllByRole("listitem");
    expect(countries[0]).toHaveTextContent("TR");
    expect(countries[0]).toHaveTextContent("3");
    expect(
      screen.getByRole("list", { name: "Top referrers" }),
    ).toHaveTextContent("github.com4");
    expect(screen.getByRole("list", { name: "Devices" })).toHaveTextContent(
      "desktop60%",
    );
  });

  it("shows an empty state without clicks", () => {
    const empty: Stats = {
      totalClicks: 0,
      byDay: [],
      byDevice: {},
      byCountry: {},
      topReferrers: [],
    };
    render(<StatsPanel code="abcd" stats={empty} onClose={vi.fn()} />);
    expect(screen.getByText(/no clicks recorded yet/i)).toBeInTheDocument();
  });

  it("closes with the button and the Escape key", async () => {
    const onClose = vi.fn();
    render(<StatsPanel code="abcd" stats={stats} onClose={onClose} />);
    await userEvent.click(screen.getByRole("button", { name: "Close" }));
    await userEvent.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalledTimes(2);
  });
});

describe("toSortedEntries", () => {
  it("sorts by count descending then name", () => {
    expect(toSortedEntries({ b: 1, a: 1, c: 5 })).toEqual([
      { name: "c", value: 5 },
      { name: "a", value: 1 },
      { name: "b", value: 1 },
    ]);
  });
});
