import { useMutation, useQueryClient } from "@tanstack/react-query";
import { ArchiveIcon, RotateCcwIcon } from "lucide-react";
import { useState } from "react";
import {
    AlertDialog,
    AlertDialogClose,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogPopup,
    AlertDialogTitle,
    AlertDialogTrigger,
} from "#/components/ui/alert-dialog";
import { Button } from "#/components/ui/button";
import { archiveTemplate, restoreTemplate } from "#/lib/api";
import type { Template } from "#/lib/api-types";

type TemplateAvailabilityActionsProps = {
    csrfToken: string;
    guildId: string;
    template: Template;
};

export function TemplateAvailabilityActions({
    csrfToken,
    guildId,
    template,
}: TemplateAvailabilityActionsProps) {
    const queryClient = useQueryClient();
    const [open, setOpen] = useState(false);
    const mutation = useMutation({
        mutationFn: () =>
            template.archived_at
                ? restoreTemplate(guildId, template.id, csrfToken)
                : archiveTemplate(guildId, template.id, csrfToken),
        onSuccess: async () => {
            await queryClient.invalidateQueries({
                queryKey: ["guild", guildId, "templates"],
            });
            setOpen(false);
        },
    });

    if (template.archived_at) {
        return (
            <div className="flex flex-col items-end gap-1">
                <Button
                    loading={mutation.isPending}
                    onClick={() => mutation.mutate()}
                    variant="outline"
                >
                    <RotateCcwIcon aria-hidden="true" />
                    Restore
                </Button>
                {mutation.isError ? (
                    <span className="text-destructive text-xs" role="alert">
                        {mutation.error.message}
                    </span>
                ) : null}
            </div>
        );
    }

    return (
        <AlertDialog onOpenChange={setOpen} open={open}>
            <AlertDialogTrigger
                render={<Button variant="destructive-outline" />}
            >
                <ArchiveIcon aria-hidden="true" />
                Archive
            </AlertDialogTrigger>
            <AlertDialogPopup>
                <AlertDialogHeader>
                    <AlertDialogTitle>
                        Archive {template.name}?
                    </AlertDialogTitle>
                    <AlertDialogDescription>
                        Moderators will no longer be able to create cases from
                        this template. Existing cases and history stay visible.
                    </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                    <AlertDialogClose render={<Button variant="ghost" />}>
                        Cancel
                    </AlertDialogClose>
                    <Button
                        loading={mutation.isPending}
                        onClick={() => mutation.mutate()}
                        variant="destructive"
                    >
                        Archive template
                    </Button>
                </AlertDialogFooter>
                {mutation.isError ? (
                    <p
                        className="px-6 pb-4 text-destructive text-xs"
                        role="alert"
                    >
                        {mutation.error.message}
                    </p>
                ) : null}
            </AlertDialogPopup>
        </AlertDialog>
    );
}
