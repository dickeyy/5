// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { TemplateFormDialog } from "./template-form-dialog";

describe("TemplateFormDialog", () => {
    it("updates controlled text fields without retaining React events", async () => {
        const queryClient = new QueryClient({
            defaultOptions: { queries: { retry: false } },
        });

        render(
            <QueryClientProvider client={queryClient}>
                <TemplateFormDialog csrfToken="csrf" guildId="guild" />
            </QueryClientProvider>,
        );

        fireEvent.click(screen.getByRole("button", { name: "New template" }));

        const name = await screen.findByRole("textbox", { name: "Name" });
        const slug = screen.getByRole("textbox", { name: "Slug" });
        const description = screen.getByRole("textbox", {
            name: "Description",
        });
        const reason = screen.getByRole("textbox", {
            name: "Official reason",
        });

        fireEvent.change(name, { target: { value: "Spam" } });
        fireEvent.change(slug, { target: { value: "spam" } });
        fireEvent.change(description, {
            target: { value: "Repeated unwanted messages" },
        });
        fireEvent.change(reason, {
            target: { value: "You repeatedly posted unwanted messages." },
        });

        expect((name as HTMLInputElement).value).toBe("Spam");
        expect((slug as HTMLInputElement).value).toBe("spam");
        expect((description as HTMLTextAreaElement).value).toBe(
            "Repeated unwanted messages",
        );
        expect((reason as HTMLTextAreaElement).value).toBe(
            "You repeatedly posted unwanted messages.",
        );
    });
});
