import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Undo2Icon } from "lucide-react";
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
import { Badge } from "#/components/ui/badge";
import { Button } from "#/components/ui/button";
import { reverseAppealAction, reverseCaseAction } from "#/lib/api";
import { titleCase } from "#/lib/format";

type ReverseActionDialogProps = {
    actionType: string;
    appealId?: string;
    caseRef: string;
    csrfToken: string;
    guildId: string;
    originalExecutionId: string;
};

export function ReverseActionDialog({
    actionType,
    appealId,
    caseRef,
    csrfToken,
    guildId,
    originalExecutionId,
}: ReverseActionDialogProps) {
    const queryClient = useQueryClient();
    const [open, setOpen] = useState(false);
    const [completed, setCompleted] = useState(false);
    const mutation = useMutation({
        mutationFn: () =>
            appealId
                ? reverseAppealAction(
                      guildId,
                      appealId,
                      originalExecutionId,
                      actionType,
                      csrfToken,
                  )
                : reverseCaseAction(
                      guildId,
                      caseRef,
                      originalExecutionId,
                      actionType,
                      csrfToken,
                  ),
        onSuccess: async () => {
            await Promise.all([
                queryClient.invalidateQueries({
                    queryKey: ["guild", guildId, "case"],
                }),
                queryClient.invalidateQueries({
                    queryKey: ["guild", guildId, "cases"],
                }),
                queryClient.invalidateQueries({
                    queryKey: ["guild", guildId, "appeals"],
                }),
                queryClient.invalidateQueries({
                    queryKey: ["guild", guildId, "action-failures"],
                }),
            ]);
            setCompleted(true);
            setOpen(false);
        },
    });

    if (completed) {
        return <Badge variant="success">Reversal queued</Badge>;
    }

    return (
        <AlertDialog onOpenChange={setOpen} open={open}>
            <AlertDialogTrigger render={<Button size="sm" variant="outline" />}>
                <Undo2Icon aria-hidden="true" />
                {titleCase(actionType)}
            </AlertDialogTrigger>
            <AlertDialogPopup>
                <AlertDialogHeader>
                    <AlertDialogTitle>
                        {titleCase(actionType)}?
                    </AlertDialogTitle>
                    <AlertDialogDescription>
                        Quack will verify your current Discord permissions and
                        queue this recovery action. The original case history
                        remains unchanged.
                    </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                    <AlertDialogClose render={<Button variant="ghost" />}>
                        Cancel
                    </AlertDialogClose>
                    <Button
                        loading={mutation.isPending}
                        onClick={() => mutation.mutate()}
                    >
                        Confirm reversal
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
