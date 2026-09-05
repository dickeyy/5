import { ExternalLinkIcon } from "lucide-react";
import { Badge, type BadgeProps } from "#/components/ui/badge";
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from "#/components/ui/card";
import { Separator } from "#/components/ui/separator";
import { ReverseActionDialog } from "#/features/actions/reverse-action-dialog";
import type { CaseDetail as CaseDetailData } from "#/lib/api-types";
import { formatDateTime, formatIdentifier, titleCase } from "#/lib/format";

type CaseDetailProps = {
    caseItem: CaseDetailData;
    reversal?: {
        caseRef: string;
        csrfToken: string;
        guildId: string;
    };
};

export function CaseDetail({ caseItem, reversal }: CaseDetailProps) {
    return (
        <div className="grid gap-4 xl:grid-cols-[minmax(0,2fr)_minmax(18rem,1fr)]">
            <div className="flex min-w-0 flex-col gap-4">
                <Card>
                    <CardHeader>
                        <CardTitle>Decision</CardTitle>
                        <CardDescription>
                            The immutable moderation record presented to staff.
                        </CardDescription>
                    </CardHeader>
                    <CardContent className="flex flex-col gap-5">
                        <p className="whitespace-pre-wrap text-sm">
                            {caseItem.reason}
                        </p>
                        <DescriptionGrid
                            items={[
                                {
                                    label: "Member",
                                    value: formatIdentifier(
                                        caseItem.target_discord_user_id,
                                    ),
                                },
                                {
                                    label: "Moderator",
                                    value: formatIdentifier(
                                        caseItem.moderator_discord_user_id,
                                    ),
                                },
                                {
                                    label: "Outcome",
                                    value:
                                        caseItem.selected_level?.name ??
                                        titleCase(
                                            caseItem.actions[0]?.action_type ??
                                                "Case only",
                                        ),
                                },
                                {
                                    label: "Source",
                                    value: titleCase(caseItem.source),
                                },
                                {
                                    label: "Created",
                                    value: formatDateTime(caseItem.created_at),
                                },
                                {
                                    label: "Template version",
                                    value: String(caseItem.template_version),
                                },
                            ]}
                        />
                    </CardContent>
                </Card>

                {caseItem.context_values?.length ||
                caseItem.context_url ||
                caseItem.context_channel_discord_id ? (
                    <Card>
                        <CardHeader>
                            <CardTitle>Context</CardTitle>
                            <CardDescription>
                                Values captured from the policy at case
                                creation.
                            </CardDescription>
                        </CardHeader>
                        <CardContent className="flex flex-col gap-4">
                            {caseItem.context_values?.map((context) => (
                                <div
                                    className="flex flex-col gap-1"
                                    key={context.key}
                                >
                                    <h3 className="font-medium text-sm">
                                        {context.label}
                                    </h3>
                                    <p className="whitespace-pre-wrap text-muted-foreground text-sm">
                                        {formatContextValue(context.value)}
                                    </p>
                                </div>
                            ))}
                            {caseItem.context_url ? (
                                <ExternalReference
                                    href={caseItem.context_url}
                                    label="Open source context"
                                />
                            ) : null}
                            {caseItem.context_channel_discord_id ? (
                                <p className="text-muted-foreground text-xs">
                                    Discord channel{" "}
                                    {caseItem.context_channel_discord_id}
                                    {caseItem.context_message_discord_id
                                        ? ` · message ${caseItem.context_message_discord_id}`
                                        : ""}
                                </p>
                            ) : null}
                        </CardContent>
                    </Card>
                ) : null}

                {caseItem.evidence.length > 0 ? (
                    <Card>
                        <CardHeader>
                            <CardTitle>Evidence</CardTitle>
                            <CardDescription>
                                Message snapshots preserved when this case was
                                created.
                            </CardDescription>
                        </CardHeader>
                        <CardContent className="flex flex-col gap-5">
                            {caseItem.evidence.map((evidence, index) => (
                                <div
                                    className="flex flex-col gap-3"
                                    key={evidence.id}
                                >
                                    {index > 0 ? <Separator /> : null}
                                    <div className="flex flex-wrap items-center gap-2">
                                        <Badge
                                            variant={statusVariant(
                                                evidence.capture_outcome,
                                            )}
                                        >
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
                                    {evidence.capture_warning ? (
                                        <p className="text-muted-foreground text-xs">
                                            {evidence.capture_warning}
                                        </p>
                                    ) : null}
                                    <ExternalReference
                                        href={evidence.message_url}
                                        label="Open Discord message"
                                    />
                                    {evidence.attachments.length > 0 ? (
                                        <ul className="flex flex-col gap-2">
                                            {evidence.attachments.map(
                                                (attachment) => (
                                                    <li
                                                        className="flex flex-wrap items-center justify-between gap-2 rounded-lg border px-3 py-2 text-sm"
                                                        key={`${evidence.id}-${attachment.filename}`}
                                                    >
                                                        <span className="min-w-0 truncate">
                                                            {
                                                                attachment.filename
                                                            }
                                                        </span>
                                                        <ExternalReference
                                                            href={
                                                                attachment.preserved_url ??
                                                                attachment.original_url
                                                            }
                                                            label="Open"
                                                        />
                                                    </li>
                                                ),
                                            )}
                                        </ul>
                                    ) : null}
                                </div>
                            ))}
                        </CardContent>
                    </Card>
                ) : null}
            </div>

            <div className="flex min-w-0 flex-col gap-4">
                {caseItem.validity === "voided" ? (
                    <Card>
                        <CardHeader>
                            <div className="flex items-center gap-2">
                                <CardTitle>Correction</CardTitle>
                                <Badge variant="secondary">Voided</Badge>
                            </div>
                            <CardDescription>
                                {caseItem.voided_at
                                    ? `Recorded ${formatDateTime(caseItem.voided_at)}`
                                    : "This case no longer counts toward escalation."}
                            </CardDescription>
                        </CardHeader>
                        <CardContent>
                            <p className="whitespace-pre-wrap text-sm">
                                {caseItem.voided_reason ??
                                    "No correction reason was returned."}
                            </p>
                        </CardContent>
                    </Card>
                ) : null}

                <Card>
                    <CardHeader>
                        <CardTitle>Enforcement</CardTitle>
                        <CardDescription>
                            Discord action and member notification status.
                        </CardDescription>
                    </CardHeader>
                    <CardContent className="flex flex-col gap-4">
                        {caseItem.actions.length === 0 ? (
                            <p className="text-muted-foreground text-sm">
                                This case did not apply a Discord action.
                            </p>
                        ) : (
                            caseItem.actions.map((action) => {
                                const reversalAction =
                                    action.status === "succeeded"
                                        ? reversalForAction(action.action_type)
                                        : null;
                                const reversalAlreadyQueued =
                                    reversalAction !== null &&
                                    caseItem.actions.some(
                                        (candidate) =>
                                            candidate.action_type ===
                                                reversalAction &&
                                            candidate.status !== "failed" &&
                                            candidate.status !== "cancelled",
                                    );
                                return (
                                    <div
                                        className="flex flex-col gap-2"
                                        key={action.id}
                                    >
                                        <div className="flex items-center justify-between gap-3">
                                            <span className="font-medium text-sm">
                                                {titleCase(action.action_type)}
                                            </span>
                                            <Badge
                                                variant={statusVariant(
                                                    action.status,
                                                )}
                                            >
                                                {titleCase(action.status)}
                                            </Badge>
                                        </div>
                                        {action.attempt_count ? (
                                            <p className="text-muted-foreground text-xs">
                                                {action.attempt_count}{" "}
                                                {action.attempt_count === 1
                                                    ? "attempt"
                                                    : "attempts"}
                                            </p>
                                        ) : null}
                                        {action.last_error ? (
                                            <p className="text-muted-foreground text-xs">
                                                {action.last_error}
                                            </p>
                                        ) : null}
                                        {reversal &&
                                        reversalAction &&
                                        !reversalAlreadyQueued ? (
                                            <div>
                                                <ReverseActionDialog
                                                    actionType={reversalAction}
                                                    caseRef={reversal.caseRef}
                                                    csrfToken={
                                                        reversal.csrfToken
                                                    }
                                                    guildId={reversal.guildId}
                                                    originalExecutionId={
                                                        action.id
                                                    }
                                                />
                                            </div>
                                        ) : null}
                                    </div>
                                );
                            })
                        )}
                        {caseItem.notification ? (
                            <>
                                <Separator />
                                <div className="flex items-center justify-between gap-3">
                                    <span className="font-medium text-sm">
                                        Member notification
                                    </span>
                                    <Badge
                                        variant={statusVariant(
                                            caseItem.notification.status,
                                        )}
                                    >
                                        {titleCase(
                                            caseItem.notification.status,
                                        )}
                                    </Badge>
                                </div>
                            </>
                        ) : null}
                    </CardContent>
                </Card>

                <Card>
                    <CardHeader>
                        <CardTitle>Timeline</CardTitle>
                        <CardDescription>
                            Durable case lifecycle events.
                        </CardDescription>
                    </CardHeader>
                    <CardContent className="flex flex-col gap-4">
                        {caseItem.events.length === 0 ? (
                            <p className="text-muted-foreground text-sm">
                                No timeline events were returned.
                            </p>
                        ) : (
                            caseItem.events.map((event) => (
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
                            ))
                        )}
                    </CardContent>
                </Card>
            </div>
        </div>
    );
}

function DescriptionGrid({
    items,
}: {
    items: Array<{ label: string; value: string }>;
}) {
    return (
        <dl className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {items.map((item) => (
                <div className="flex min-w-0 flex-col gap-1" key={item.label}>
                    <dt className="text-muted-foreground text-xs">
                        {item.label}
                    </dt>
                    <dd className="truncate text-sm">{item.value}</dd>
                </div>
            ))}
        </dl>
    );
}

function ExternalReference({ href, label }: { href: string; label: string }) {
    if (!isSafeExternalUrl(href)) {
        return <span className="text-muted-foreground text-xs">{label}</span>;
    }

    return (
        <a
            className="inline-flex w-fit items-center gap-1 font-medium text-sm underline-offset-4 hover:underline"
            href={href}
            rel="noreferrer"
            target="_blank"
        >
            {label}
            <ExternalLinkIcon aria-hidden="true" className="size-3.5" />
        </a>
    );
}

function formatContextValue(value: unknown): string {
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

function isSafeExternalUrl(value: string): boolean {
    try {
        const url = new URL(value);
        return url.protocol === "https:" || url.protocol === "http:";
    } catch {
        return false;
    }
}

function statusVariant(status: string): BadgeProps["variant"] {
    if (["sent", "succeeded", "success", "preserved"].includes(status)) {
        return "success";
    }
    if (["failed", "failure"].includes(status)) {
        return "error";
    }
    if (["pending", "running", "retrying", "prepared"].includes(status)) {
        return "warning";
    }
    return "secondary";
}

function reversalForAction(actionType: string): string | null {
    if (actionType === "timeout_user") {
        return "remove_timeout";
    }
    if (actionType === "ban_user") {
        return "unban_user";
    }
    return null;
}
