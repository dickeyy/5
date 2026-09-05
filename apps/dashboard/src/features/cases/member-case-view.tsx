import { ExternalLinkIcon } from "lucide-react";
import { Alert, AlertDescription, AlertTitle } from "#/components/ui/alert";
import { Badge } from "#/components/ui/badge";
import {
    Card,
    CardDescription,
    CardHeader,
    CardPanel,
    CardTitle,
} from "#/components/ui/card";
import { Separator } from "#/components/ui/separator";
import type { MemberCase } from "#/lib/api-types";
import { formatDateTime, titleCase } from "#/lib/format";

export function MemberCaseView({ caseItem }: { caseItem: MemberCase }) {
    return (
        <div className="grid gap-4 lg:grid-cols-[minmax(0,2fr)_minmax(18rem,1fr)]">
            <div className="flex flex-col gap-4">
                <Card>
                    <CardHeader>
                        <CardTitle>Official reason</CardTitle>
                        <CardDescription>
                            Recorded {formatDateTime(caseItem.created_at)}
                        </CardDescription>
                    </CardHeader>
                    <CardPanel className="flex flex-col gap-5">
                        <p className="whitespace-pre-wrap text-sm">
                            {caseItem.official_reason}
                        </p>
                        {caseItem.context.length > 0 ? (
                            <>
                                <Separator />
                                <section className="flex flex-col gap-4">
                                    <h2 className="font-medium text-sm">
                                        Context
                                    </h2>
                                    {caseItem.context.map((context) => (
                                        <div
                                            className="flex flex-col gap-1"
                                            key={context.key}
                                        >
                                            <h3 className="font-medium text-sm">
                                                {context.label}
                                            </h3>
                                            <p className="whitespace-pre-wrap text-muted-foreground text-sm">
                                                {formatValue(context.value)}
                                            </p>
                                        </div>
                                    ))}
                                </section>
                            </>
                        ) : null}
                    </CardPanel>
                </Card>
                {caseItem.evidence.length > 0 ? (
                    <Card>
                        <CardHeader>
                            <CardTitle>Evidence</CardTitle>
                            <CardDescription>
                                Message snapshots attached to this case.
                            </CardDescription>
                        </CardHeader>
                        <CardPanel className="flex flex-col gap-5">
                            {caseItem.evidence.map((evidence, index) => (
                                <div
                                    className="flex flex-col gap-3"
                                    key={evidence.id}
                                >
                                    {index > 0 ? <Separator /> : null}
                                    <div className="flex flex-wrap items-center gap-2">
                                        <Badge variant="secondary">
                                            {titleCase(
                                                evidence.capture_outcome,
                                            )}
                                        </Badge>
                                        <span className="text-muted-foreground text-xs">
                                            {formatDateTime(
                                                evidence.message_created_at,
                                            )}
                                        </span>
                                    </div>
                                    <p className="whitespace-pre-wrap text-sm">
                                        {evidence.content ||
                                            "No message content was captured."}
                                    </p>
                                    {safeUrl(evidence.message_url) ? (
                                        <a
                                            className="inline-flex w-fit items-center gap-1 font-medium text-sm underline-offset-4 hover:underline"
                                            href={evidence.message_url}
                                            rel="noreferrer"
                                            target="_blank"
                                        >
                                            Open Discord message
                                            <ExternalLinkIcon
                                                aria-hidden="true"
                                                className="size-3.5"
                                            />
                                        </a>
                                    ) : null}
                                </div>
                            ))}
                        </CardPanel>
                    </Card>
                ) : null}
            </div>
            <div className="flex flex-col gap-4">
                <Card>
                    <CardHeader>
                        <div className="flex items-center gap-2">
                            <CardTitle>Outcome</CardTitle>
                            <Badge
                                variant={
                                    caseItem.validity === "voided"
                                        ? "secondary"
                                        : "success"
                                }
                            >
                                {titleCase(caseItem.validity)}
                            </Badge>
                        </div>
                        <CardDescription>
                            {caseItem.selected_outcome?.name ?? "Case recorded"}
                        </CardDescription>
                    </CardHeader>
                    <CardPanel className="flex flex-col gap-3">
                        {caseItem.enforcement ? (
                            <div className="flex items-center justify-between gap-3 text-sm">
                                <span>
                                    {titleCase(
                                        caseItem.enforcement.action_type,
                                    )}
                                </span>
                                <Badge variant="secondary">
                                    {titleCase(caseItem.enforcement.status)}
                                </Badge>
                            </div>
                        ) : (
                            <p className="text-muted-foreground text-sm">
                                No additional Discord action was configured.
                            </p>
                        )}
                        {caseItem.voided_reason ? (
                            <Alert>
                                <AlertTitle>Case correction</AlertTitle>
                                <AlertDescription>
                                    {caseItem.voided_reason}
                                </AlertDescription>
                            </Alert>
                        ) : null}
                    </CardPanel>
                </Card>
                {caseItem.history.length > 0 ? (
                    <Card>
                        <CardHeader>
                            <CardTitle>History</CardTitle>
                            <CardDescription>
                                Public updates for this case.
                            </CardDescription>
                        </CardHeader>
                        <CardPanel className="flex flex-col gap-4">
                            {caseItem.history.map((event) => (
                                <div
                                    className="flex flex-col gap-1 border-l pl-3"
                                    key={event.id}
                                >
                                    <div className="flex flex-wrap items-center justify-between gap-2">
                                        <span className="font-medium text-sm">
                                            {titleCase(event.event_type)}
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
                        </CardPanel>
                    </Card>
                ) : null}
            </div>
        </div>
    );
}

function formatValue(value: unknown): string {
    if (typeof value === "boolean") {
        return value ? "Yes" : "No";
    }
    if (typeof value === "number") {
        return String(value);
    }
    if (typeof value === "string" && value.trim()) {
        return value;
    }
    return "No value";
}

function safeUrl(value: string): boolean {
    try {
        const url = new URL(value);
        return url.protocol === "https:" || url.protocol === "http:";
    } catch {
        return false;
    }
}
