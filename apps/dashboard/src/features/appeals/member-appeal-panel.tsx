import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, AlertDescription, AlertTitle } from "#/components/ui/alert";
import { Badge } from "#/components/ui/badge";
import { Button } from "#/components/ui/button";
import {
    Card,
    CardDescription,
    CardHeader,
    CardPanel,
    CardTitle,
} from "#/components/ui/card";
import { Field, FieldDescription, FieldLabel } from "#/components/ui/field";
import { Separator } from "#/components/ui/separator";
import { Textarea } from "#/components/ui/textarea";
import { submitAppealInformation } from "#/lib/api";
import { formatDateTime, titleCase } from "#/lib/format";
import { memberAppealQuery } from "#/lib/queries";

type MemberAppealPanelProps = {
    appealId: string;
    csrfToken: string;
};

export function MemberAppealPanel({
    appealId,
    csrfToken,
}: MemberAppealPanelProps) {
    const queryClient = useQueryClient();
    const appeal = useQuery(memberAppealQuery(appealId));
    const mutation = useMutation({
        mutationFn: (body: string) =>
            submitAppealInformation(appealId, body, csrfToken),
        onSuccess: async () => {
            await queryClient.invalidateQueries({
                queryKey: ["member", "appeal", appealId],
            });
        },
    });

    if (appeal.isPending) {
        return (
            <Card>
                <CardHeader>
                    <CardTitle>Appeal</CardTitle>
                    <CardDescription>Loading appeal status…</CardDescription>
                </CardHeader>
            </Card>
        );
    }

    if (appeal.isError) {
        return (
            <Alert variant="error">
                <AlertTitle>Appeal could not be loaded</AlertTitle>
                <AlertDescription>{appeal.error.message}</AlertDescription>
            </Alert>
        );
    }

    return (
        <Card>
            <CardHeader>
                <div className="flex items-center gap-2">
                    <CardTitle>Appeal</CardTitle>
                    <Badge variant={appealStatusVariant(appeal.data.status)}>
                        {titleCase(appeal.data.status)}
                    </Badge>
                </div>
                <CardDescription>
                    Submitted {formatDateTime(appeal.data.created_at)}
                </CardDescription>
            </CardHeader>
            <CardPanel className="flex flex-col gap-5">
                <section className="flex flex-col gap-4">
                    {appeal.data.questions
                        .slice()
                        .sort((left, right) => left.position - right.position)
                        .map((question) => {
                            const answer = appeal.data.answers.find(
                                (item) => item.question_id === question.id,
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
                {appeal.data.decision_reason ? (
                    <>
                        <Separator />
                        <section className="flex flex-col gap-1">
                            <h3 className="font-medium text-sm">
                                Staff decision
                            </h3>
                            <p className="whitespace-pre-wrap text-muted-foreground text-sm">
                                {appeal.data.decision_reason}
                            </p>
                        </section>
                    </>
                ) : null}
                {appeal.data.events.length > 0 ? (
                    <>
                        <Separator />
                        <section className="flex flex-col gap-3">
                            <h3 className="font-medium text-sm">Timeline</h3>
                            {appeal.data.events.map((event) => (
                                <div
                                    className="flex flex-col gap-1 border-l pl-3"
                                    key={event.id}
                                >
                                    <div className="flex flex-wrap items-center justify-between gap-2">
                                        <span className="font-medium text-sm">
                                            {titleCase(event.type)}
                                        </span>
                                        <span className="text-muted-foreground text-xs">
                                            {formatDateTime(event.created_at)}
                                        </span>
                                    </div>
                                    {event.body ? (
                                        <p className="whitespace-pre-wrap text-muted-foreground text-sm">
                                            {event.body}
                                        </p>
                                    ) : null}
                                </div>
                            ))}
                        </section>
                    </>
                ) : null}
                {appeal.data.status === "needs_information" ? (
                    <>
                        <Separator />
                        <form
                            className="flex flex-col gap-4"
                            onSubmit={(event) => {
                                event.preventDefault();
                                const formData = new FormData(
                                    event.currentTarget,
                                );
                                const body = String(
                                    formData.get("body") ?? "",
                                ).trim();
                                if (body) {
                                    mutation.mutate(body);
                                }
                            }}
                        >
                            <Field name="body">
                                <FieldLabel>Additional information</FieldLabel>
                                <Textarea
                                    maxLength={4000}
                                    name="body"
                                    required
                                />
                                <FieldDescription>
                                    Respond to the latest request from staff.
                                </FieldDescription>
                            </Field>
                            {mutation.isError ? (
                                <Alert variant="error">
                                    <AlertTitle>
                                        Information was not submitted
                                    </AlertTitle>
                                    <AlertDescription>
                                        {mutation.error.message}
                                    </AlertDescription>
                                </Alert>
                            ) : null}
                            <div className="flex justify-end">
                                <Button
                                    loading={mutation.isPending}
                                    type="submit"
                                >
                                    Submit information
                                </Button>
                            </div>
                        </form>
                    </>
                ) : null}
            </CardPanel>
        </Card>
    );
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

function appealStatusVariant(status: string) {
    switch (status) {
        case "accepted":
            return "success" as const;
        case "needs_information":
            return "warning" as const;
        case "rejected":
        case "closed":
            return "secondary" as const;
        default:
            return "info" as const;
    }
}
