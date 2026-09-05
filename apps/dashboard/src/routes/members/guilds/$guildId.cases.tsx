import { useQuery } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";
import { BirdIcon, ClipboardListIcon } from "lucide-react";
import { QueryError, QueryPending } from "#/components/dashboard/query-state";
import { Badge } from "#/components/ui/badge";
import { Button } from "#/components/ui/button";
import {
    Card,
    CardAction,
    CardDescription,
    CardHeader,
    CardTitle,
} from "#/components/ui/card";
import {
    Empty,
    EmptyDescription,
    EmptyHeader,
    EmptyMedia,
    EmptyTitle,
} from "#/components/ui/empty";
import { SignOutButton } from "#/features/auth/sign-out-button";
import { formatDateTime } from "#/lib/format";
import { memberCasesQuery, sessionQuery } from "#/lib/queries";

export const Route = createFileRoute("/members/guilds/$guildId/cases")({
    component: MemberCasesPage,
});

function MemberCasesPage() {
    const { guildId } = Route.useParams();
    const cases = useQuery(memberCasesQuery(guildId));
    const session = useQuery(sessionQuery);

    if (cases.isPending || session.isPending) {
        return <QueryPending label="Loading your cases" />;
    }

    if (cases.isError || session.isError) {
        return (
            <main className="mx-auto flex min-h-svh w-full max-w-2xl items-center px-6">
                <QueryError
                    onRetry={() => {
                        cases.refetch();
                        session.refetch();
                    }}
                />
            </main>
        );
    }

    return (
        <main className="mx-auto flex min-h-svh w-full max-w-4xl flex-col gap-8 px-4 py-8 sm:px-6 sm:py-12">
            <header className="flex flex-wrap items-center justify-between gap-4">
                <div className="flex items-center gap-3">
                    <div className="flex size-9 items-center justify-center rounded-lg border bg-card shadow-xs/5">
                        <BirdIcon aria-hidden="true" />
                    </div>
                    <div>
                        <p className="font-heading font-semibold">Quack</p>
                        <p className="text-muted-foreground text-xs">
                            Signed in as{" "}
                            {session.data.user.global_name ||
                                session.data.user.username}
                        </p>
                    </div>
                </div>
                <SignOutButton />
            </header>
            <div className="flex flex-col gap-1">
                <h1 className="font-heading font-semibold text-2xl tracking-tight">
                    Your cases
                </h1>
                <p className="text-muted-foreground text-sm">
                    Moderation records in this Discord server.
                </p>
            </div>
            {cases.data.cases.length === 0 ? (
                <Empty className="rounded-2xl border">
                    <EmptyMedia variant="icon">
                        <ClipboardListIcon aria-hidden="true" />
                    </EmptyMedia>
                    <EmptyHeader>
                        <EmptyTitle>No cases</EmptyTitle>
                        <EmptyDescription>
                            This server has no moderation cases for your Discord
                            account.
                        </EmptyDescription>
                    </EmptyHeader>
                </Empty>
            ) : (
                <section
                    aria-label="Your moderation cases"
                    className="grid gap-3"
                >
                    {cases.data.cases.map((caseItem) => (
                        <Card key={caseItem.id}>
                            <CardHeader>
                                <div className="flex min-w-0 flex-wrap items-center gap-2">
                                    <CardTitle className="text-base">
                                        Case #{caseItem.case_number}
                                    </CardTitle>
                                    <Badge
                                        variant={
                                            caseItem.validity === "voided"
                                                ? "secondary"
                                                : "success"
                                        }
                                    >
                                        {caseItem.validity === "voided"
                                            ? "Voided"
                                            : "Valid"}
                                    </Badge>
                                    {caseItem.appeal_status ? (
                                        <Badge variant="outline">
                                            Appeal{" "}
                                            {caseItem.appeal_status.replaceAll(
                                                "_",
                                                " ",
                                            )}
                                        </Badge>
                                    ) : null}
                                </div>
                                <CardDescription className="line-clamp-2 max-w-2xl">
                                    {caseItem.official_reason}
                                </CardDescription>
                                <p className="text-muted-foreground text-xs">
                                    {formatDateTime(caseItem.created_at)}
                                </p>
                                <CardAction>
                                    <Button
                                        render={
                                            <Link
                                                params={{
                                                    caseRef: caseItem.id,
                                                    guildId,
                                                }}
                                                to="/guilds/$guildId/cases/$caseRef/appeal"
                                            />
                                        }
                                        size="sm"
                                        variant="outline"
                                    >
                                        View
                                    </Button>
                                </CardAction>
                            </CardHeader>
                        </Card>
                    ))}
                </section>
            )}
        </main>
    );
}
