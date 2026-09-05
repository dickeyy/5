import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Alert, AlertDescription, AlertTitle } from "#/components/ui/alert";
import { Button } from "#/components/ui/button";
import {
    Dialog,
    DialogClose,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogPanel,
    DialogPopup,
    DialogTitle,
    DialogTrigger,
} from "#/components/ui/dialog";
import { Field, FieldDescription, FieldLabel } from "#/components/ui/field";
import { Textarea } from "#/components/ui/textarea";
import { voidCase } from "#/lib/api";

type VoidCaseDialogProps = {
    caseRef: string;
    csrfToken: string;
    guildId: string;
};

export function VoidCaseDialog({
    caseRef,
    csrfToken,
    guildId,
}: VoidCaseDialogProps) {
    const queryClient = useQueryClient();
    const [open, setOpen] = useState(false);
    const mutation = useMutation({
        mutationFn: (reason: string) =>
            voidCase(guildId, caseRef, reason, csrfToken),
        onSuccess: async () => {
            await Promise.all([
                queryClient.invalidateQueries({
                    queryKey: ["guild", guildId, "case", caseRef],
                }),
                queryClient.invalidateQueries({
                    queryKey: ["guild", guildId, "cases"],
                }),
                queryClient.invalidateQueries({
                    queryKey: ["guild", guildId, "statistics"],
                }),
            ]);
            setOpen(false);
        },
    });

    return (
        <Dialog onOpenChange={setOpen} open={open}>
            <DialogTrigger render={<Button variant="destructive-outline" />}>
                Void case
            </DialogTrigger>
            <DialogPopup>
                <DialogHeader>
                    <DialogTitle>Void this case?</DialogTitle>
                    <DialogDescription>
                        The case stays in the audit history but no longer counts
                        toward future escalation.
                    </DialogDescription>
                </DialogHeader>
                <form
                    className="contents"
                    onSubmit={(event) => {
                        event.preventDefault();
                        const formData = new FormData(event.currentTarget);
                        const reason = String(
                            formData.get("reason") ?? "",
                        ).trim();
                        if (reason) {
                            mutation.mutate(reason);
                        }
                    }}
                >
                    <DialogPanel>
                        <Field name="reason">
                            <FieldLabel>Correction reason</FieldLabel>
                            <Textarea
                                autoFocus
                                maxLength={2000}
                                name="reason"
                                required
                            />
                            <FieldDescription>
                                Explain why the immutable case record is being
                                corrected.
                            </FieldDescription>
                        </Field>
                        {mutation.isError ? (
                            <Alert className="mt-4" variant="error">
                                <AlertTitle>Case was not voided</AlertTitle>
                                <AlertDescription>
                                    {mutation.error.message}
                                </AlertDescription>
                            </Alert>
                        ) : null}
                    </DialogPanel>
                    <DialogFooter>
                        <DialogClose render={<Button variant="ghost" />}>
                            Cancel
                        </DialogClose>
                        <Button
                            loading={mutation.isPending}
                            type="submit"
                            variant="destructive"
                        >
                            Void case
                        </Button>
                    </DialogFooter>
                </form>
            </DialogPopup>
        </Dialog>
    );
}
