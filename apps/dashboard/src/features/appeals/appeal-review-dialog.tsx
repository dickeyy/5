import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Alert, AlertDescription, AlertTitle } from "#/components/ui/alert";
import { Badge } from "#/components/ui/badge";
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
import {
    Select,
    SelectItem,
    SelectPopup,
    SelectTrigger,
    SelectValue,
} from "#/components/ui/select";
import { Separator } from "#/components/ui/separator";
import { Textarea } from "#/components/ui/textarea";
import { ReverseActionDialog } from "#/features/actions/reverse-action-dialog";
import { transitionAppeal } from "#/lib/api";
import type { Appeal, AppealTransition } from "#/lib/api-types";
import { formatDateTime, titleCase } from "#/lib/format";

type AppealReviewDialogProps = {
    appeal: Appeal;
    csrfToken: string;
    guildId: string;
};

export function AppealReviewDialog({
    appeal,
    csrfToken,
    guildId,
}: AppealReviewDialogProps) {
    const queryClient = useQueryClient();
    const [open, setOpen] = useState(false);
    const transitions = availableTransitions(appeal.status);
    const [decision, setDecision] = useState<AppealTransition | null>(
        transitions[0]?.value ?? null,
    );
    const mutation = useMutation({
        mutationFn: (input: { transition: AppealTransition; reason: string }) =>
            transitionAppeal(
                guildId,
                appeal.id,
                input.transition,
                input.reason,
                csrfToken,
            ),
        onSuccess: async () => {
            await Promise.all([
                queryClient.invalidateQueries({
                    queryKey: ["guild", guildId, "appeals"],
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
            <DialogTrigger render={<Button size="sm" variant="outline" />}>
                Review
            </DialogTrigger>
            <DialogPopup className="max-w-2xl">
                <DialogHeader>
                    <DialogTitle>Appeal review</DialogTitle>
                    <DialogDescription>
                        Review the member’s submitted answers and record a
                        public-safe staff decision.
                    </DialogDescription>
                </DialogHeader>
                <form
                    className="contents"
                    onSubmit={(event) => {
                        event.preventDefault();
                        if (!decision) {
                            return;
                        }
                        const formData = new FormData(event.currentTarget);
                        mutation.mutate({
                            reason: String(formData.get("reason") ?? "").trim(),
                            transition: decision,
                        });
                    }}
                >
                    <DialogPanel className="flex flex-col gap-5">
                        <div className="flex flex-wrap items-center gap-2 text-sm">
                            <Badge variant={statusVariant(appeal.status)}>
                                {titleCase(appeal.status)}
                            </Badge>
                            <span className="text-muted-foreground">
                                Submitted {formatDateTime(appeal.created_at)}
                            </span>
                        </div>
                        <section className="flex flex-col gap-4">
                            {appeal.questions
                                .slice()
                                .sort(
                                    (left, right) =>
                                        left.position - right.position,
                                )
                                .map((question) => {
                                    const answer = appeal.answers.find(
                                        (item) =>
                                            item.question_id === question.id,
                                    );
                                    return (
                                        <div
                                            className="flex flex-col gap-1"
                                            key={question.id}
                                        >
                                            <h3 className="font-medium text-sm">
                                                {question.prompt}
                                            </h3>
                                            <p className="whitespace-pre-wrap text-muted-foreground text-sm">
                                                {formatAnswer(answer?.value)}
                                            </p>
                                        </div>
                                    );
                                })}
                        </section>
                        {appeal.events.length > 0 ? (
                            <>
                                <Separator />
                                <section className="flex flex-col gap-3">
                                    <h3 className="font-medium text-sm">
                                        Timeline
                                    </h3>
                                    {appeal.events.map((event) => (
                                        <div
                                            className="flex flex-col gap-1 rounded-lg border px-3 py-2"
                                            key={event.id}
                                        >
                                            <div className="flex items-center justify-between gap-4 text-xs">
                                                <span className="font-medium">
                                                    {titleCase(event.type)}
                                                </span>
                                                <span className="text-muted-foreground">
                                                    {formatDateTime(
                                                        event.created_at,
                                                    )}
                                                </span>
                                            </div>
                                            {event.body ? (
                                                <p className="text-muted-foreground text-sm">
                                                    {event.body}
                                                </p>
                                            ) : null}
                                        </div>
                                    ))}
                                </section>
                            </>
                        ) : null}
                        {appeal.status === "accepted" &&
                        appeal.reversal_offers?.length ? (
                            <>
                                <Separator />
                                <section className="flex flex-col gap-3">
                                    <div>
                                        <h3 className="font-medium text-sm">
                                            Discord action recovery
                                        </h3>
                                        <p className="text-muted-foreground text-xs">
                                            Accepting an appeal voids the case.
                                            Reverse eligible Discord actions
                                            separately.
                                        </p>
                                    </div>
                                    <div className="flex flex-wrap gap-2">
                                        {appeal.reversal_offers.map((offer) => (
                                            <ReverseActionDialog
                                                actionType={offer.action_type}
                                                appealId={appeal.id}
                                                caseRef={appeal.case_id}
                                                csrfToken={csrfToken}
                                                guildId={guildId}
                                                key={
                                                    offer.original_execution_id
                                                }
                                                originalExecutionId={
                                                    offer.original_execution_id
                                                }
                                            />
                                        ))}
                                    </div>
                                </section>
                            </>
                        ) : null}
                        {transitions.length > 0 ? (
                            <>
                                <Separator />
                                <div className="grid gap-4 sm:grid-cols-2">
                                    <Field name="decision">
                                        <FieldLabel>Decision</FieldLabel>
                                        <Select
                                            items={transitions}
                                            onValueChange={(value) =>
                                                setDecision(
                                                    value as AppealTransition,
                                                )
                                            }
                                            value={decision}
                                        >
                                            <SelectTrigger>
                                                <SelectValue />
                                            </SelectTrigger>
                                            <SelectPopup>
                                                {transitions.map((item) => (
                                                    <SelectItem
                                                        key={item.value}
                                                        value={item.value}
                                                    >
                                                        {item.label}
                                                    </SelectItem>
                                                ))}
                                            </SelectPopup>
                                        </Select>
                                        {decision === "accept" ? (
                                            <FieldDescription>
                                                Accepting also voids the case.
                                                Discord action reversal remains
                                                separate.
                                            </FieldDescription>
                                        ) : null}
                                    </Field>
                                    <Field name="reason">
                                        <FieldLabel>Reason</FieldLabel>
                                        <Textarea
                                            maxLength={2000}
                                            name="reason"
                                            required
                                        />
                                        <FieldDescription>
                                            Visible to the affected member.
                                        </FieldDescription>
                                    </Field>
                                </div>
                            </>
                        ) : null}
                        {mutation.isError ? (
                            <Alert variant="error">
                                <AlertTitle>Appeal was not updated</AlertTitle>
                                <AlertDescription>
                                    {mutation.error.message}
                                </AlertDescription>
                            </Alert>
                        ) : null}
                    </DialogPanel>
                    <DialogFooter>
                        <DialogClose render={<Button variant="ghost" />}>
                            Close
                        </DialogClose>
                        {transitions.length > 0 ? (
                            <Button loading={mutation.isPending} type="submit">
                                Apply decision
                            </Button>
                        ) : null}
                    </DialogFooter>
                </form>
            </DialogPopup>
        </Dialog>
    );
}

function availableTransitions(status: string) {
    switch (status) {
        case "pending":
            return [
                { label: "Accept appeal", value: "accept" },
                {
                    label: "Request information",
                    value: "request-information",
                },
                { label: "Reject appeal", value: "reject" },
                { label: "Close appeal", value: "close" },
            ] satisfies Array<{ label: string; value: AppealTransition }>;
        case "needs_information":
            return [{ label: "Close appeal", value: "close" }] satisfies Array<{
                label: string;
                value: AppealTransition;
            }>;
        case "rejected":
        case "closed":
            return [
                { label: "Reopen appeal", value: "reopen" },
            ] satisfies Array<{ label: string; value: AppealTransition }>;
        default:
            return [] satisfies Array<{
                label: string;
                value: AppealTransition;
            }>;
    }
}

function formatAnswer(value: unknown): string {
    if (typeof value === "boolean") {
        return value ? "Yes" : "No";
    }
    if (typeof value === "string" && value.trim()) {
        return value;
    }
    return "No answer provided";
}

function statusVariant(status: string) {
    switch (status) {
        case "accepted":
            return "success" as const;
        case "needs_information":
            return "warning" as const;
        case "pending":
            return "info" as const;
        default:
            return "secondary" as const;
    }
}
