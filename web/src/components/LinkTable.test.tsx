import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { LinkTable } from "./LinkTable";
import type { Link } from "../types";
const links: Link[] = [
  {
    code: "abcd",
    shortUrl: "http://x/abcd",
    url: "https://example.com/one",
    custom: true,
    createdAt: "2026-05-02T10:00:00Z",
    expired: false,
    clicks: 42,
  },
  {
    code: "1001",
    shortUrl: "http://x/1001",
    url: "https://example.com/two",
    custom: false,
    createdAt: "2026-05-01T10:00:00Z",
    expiresAt: "2026-05-01T11:00:00Z",
    expired: true,
    clicks: 0,
  },
];
describe("LinkTable", () => {
  it("renders an empty state", () => {
    render(
      <LinkTable
        links={[]}
        selected={null}
        onSelect={vi.fn()}
        onDelete={vi.fn()}
      />,
    );
    expect(screen.getByText(/no links yet/i)).toBeInTheDocument();
  });
  it("renders rows with click counts and badges and wires the actions", async () => {
    const onSelect = vi.fn();
    const onDelete = vi.fn();
    render(
      <LinkTable
        links={links}
        selected="abcd"
        onSelect={onSelect}
        onDelete={onDelete}
      />,
    );
    const rows = screen.getAllByRole("row").slice(1);
    expect(rows).toHaveLength(2);
    expect(within(rows[0]).getByText("42")).toBeInTheDocument();
    expect(within(rows[0]).getByText("active")).toBeInTheDocument();
    expect(within(rows[0]).getByText("alias")).toBeInTheDocument();
    expect(within(rows[1]).getByText("expired")).toBeInTheDocument();
    expect(rows[0]).toHaveClass("selected");
    await userEvent.click(
      within(rows[1]).getByRole("button", { name: "Stats" }),
    );
    expect(onSelect).toHaveBeenCalledWith("1001");
    await userEvent.click(screen.getByRole("button", { name: "Delete abcd" }));
    expect(onDelete).toHaveBeenCalledWith("abcd");
  });

  it("filters rows by code or destination", async () => {
    render(
      <LinkTable
        links={links}
        selected={null}
        onSelect={vi.fn()}
        onDelete={vi.fn()}
      />,
    );
    await userEvent.type(
      screen.getByRole("searchbox", { name: "Filter links" }),
      "two",
    );
    expect(screen.getAllByRole("row").slice(1)).toHaveLength(1);
    expect(screen.getByText("/1001")).toBeInTheDocument();

    await userEvent.clear(
      screen.getByRole("searchbox", { name: "Filter links" }),
    );
    await userEvent.type(
      screen.getByRole("searchbox", { name: "Filter links" }),
      "nothing-like-this",
    );
    expect(screen.getByText(/no links match/i)).toBeInTheDocument();
  });
});
