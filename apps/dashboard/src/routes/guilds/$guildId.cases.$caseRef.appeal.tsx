import { useQuery } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";
import { BirdIcon } from "lucide-react";
import { QueryError, QueryPending } from "#/components/dashboard/query-state";
import { Alert, AlertDescription, AlertTitle } from "#/components/ui/alert";
import { Badge } from "#/components/ui/badge";
import { Button } from "#/components/ui/button";
import { MemberAppealPanel } from "#/features/appeals/member-appeal-panel";
import { SignOutButton } from "#/features/auth/sign-out-button";
import { MemberCaseView } from "#/features/cases/member-case-view";
import { memberCaseQuery, sessionQuery } from "#/lib/queries";

export const Route = createFileRoute("/guilds/$guildId/cases/$caseRef/appeal")({
    component: MemberCaseAppealPage,
});

function MemberCaseAppealPage() {
    const { caseRef } = Route.useParams();
    const caseResult = useQuery(memberCaseQuery(caseRef));
    const session = useQuery(sessionQuery);

    if (caseResult.isPending || session.isPending) {
        return <QueryPending label="Loading your case" />;
    }

    if (caseResult.isError || session.isError) {
        return (
            <main className="mx-auto flex min-h-svh w-full max-w-2xl items-center px-6">
                <QueryError
                    description="This case is unavailable or does not belong to your Discord account."
                    onRetry={() => {
                        caseResult.refetch();
                        session.refetch();
                    }}
                />
            </main>
        );
    }

    const caseItem = caseResult.data;

    return (
        <main className="mx-auto flex min-h-svh w-full max-w-6xl flex-col gap-8 px-4 py-8 sm:px-6 sm:py-12">
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
                <div className="flex items-center gap-2">
                    <Button
                        render={
                            <Link
                                params={{ guildId: caseItem.guild_id }}
                                to="/members/guilds/$guildId/cases"
                            />
                        }
                        size="sm"
                        variant="ghost"
                    >
                        My cases
                    </Button>
                    <SignOutButton />
                </div>
            </header>
            <div className="flex flex-wrap items-end justify-between gap-4">
                <div className="flex flex-col gap-1">
                    <h1 className="font-heading font-semibold text-2xl tracking-tight">
                        Case #{caseItem.case_number}
                    </h1>
                    <p className="text-muted-foreground text-sm">
                        Moderation record for your Discord account.
                    </p>
                </div>
                <Badge
                    variant={
                        caseItem.validity === "voided" ? "secondary" : "success"
                    }
                >
                    {caseItem.validity === "voided" ? "Voided" : "Valid"}
                </Badge>
            </div>
            <MemberCaseView caseItem={caseItem} />
            {caseItem.appeal_id ? (
                <MemberAppealPanel
                    appealId={caseItem.appeal_id}
                    csrfToken={session.data.csrf_token}
                />
            ) : caseItem.appealable ? (
                <Alert variant="warning">
                    <AlertTitle>Appeal form temporarily unavailable</AlertTitle>
                    <AlertDescription>
                        Quack could not load this server’s configured appeal
                        questions. Your case remains eligible; please try again
                        later.
                    </AlertDescription>
                </Alert>
            ) : (
                <Alert>
                    <AlertTitle>No appeal available</AlertTitle>
                    <AlertDescription>
                        This case is not currently eligible for a new appeal.
                    </AlertDescription>
                </Alert>
            )}
        </main>
    );
}
