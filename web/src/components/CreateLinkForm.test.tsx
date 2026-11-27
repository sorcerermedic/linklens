import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { CreateLinkForm } from "./CreateLinkForm";
import type { Link } from "../types";
const link: Link = {
  code: "launch",
  shortUrl: "http://localhost:8080/launch",
  url: "https://example.com",
  custom: true,
  createdAt: "2026-05-01T10:00:00Z",
  expired: false,
  clicks: 0,
};
describe("CreateLinkForm", () => {
  it("submits url and alias and shows the created short link", async () => {
    const onCreate = vi.fn().mockResolvedValue(link);
    render(<CreateLinkForm onCreate={onCreate} />);
    await userEvent.type(
      screen.getByLabelText(/destination url/i),
      "https://example.com",
    );
    await userEvent.type(screen.getByLabelText(/custom alias/i), "launch");
    await userEvent.click(
      screen.getByRole("button", { name: /create short link/i }),
    );
    expect(onCreate).toHaveBeenCalledWith({
      url: "https://example.com",
      alias: "launch",
    });
    expect(await screen.findByRole("status")).toHaveTextContent(
      "http://localhost:8080/launch",
    );
    expect(screen.getByLabelText(/destination url/i)).toHaveValue("");
  });
  it("shows the server error message when creation fails", async () => {
    const onCreate = vi
      .fn()
      .mockRejectedValue(new Error("alias already taken"));
    render(<CreateLinkForm onCreate={onCreate} />);
    await userEvent.type(
      screen.getByLabelText(/destination url/i),
      "https://example.com",
    );
    await userEvent.click(
      screen.getByRole("button", { name: /create short link/i }),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "alias already taken",
    );
  });
  it("validates that a url was entered before calling the api", async () => {
    const onCreate = vi.fn();
    render(<CreateLinkForm onCreate={onCreate} />);
    await userEvent.click(
      screen.getByRole("button", { name: /create short link/i }),
    );
    expect(onCreate).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent(/enter a url/i);
  });
});
