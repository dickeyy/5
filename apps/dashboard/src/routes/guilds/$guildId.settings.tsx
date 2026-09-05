import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { PageHeader } from "#/components/dashboard/page-header";
import { QueryError, QueryPending } from "#/components/dashboard/query-state";
import { Alert, AlertDescription, AlertTitle } from "#/components/ui/alert";
import { AppealSettingsForm } from "#/features/settings/appeal-settings-form";
import { SettingsForm } from "#/features/settings/settings-form";
import { StarterPolicyNotice } from "#/features/settings/starter-policy-notice";
import {
    appealSettingsQuery,
    guildContextQuery,
    guildSettingsQuery,
    sessionQuery,
} from "#/lib/queries";

export const Route = createFileRoute("/guilds/$guildId/settings")({
    component: SettingsPage,
});

function SettingsPage() {
    const { guildId } = Route.useParams();
    const settings = useQuery(guildSettingsQuery(guildId));
    const appealSettings = useQuery(appealSettingsQuery(guildId));
    const guildContext = useQuery(guildContextQuery(guildId));
    const session = useQuery(sessionQuery);

    return (
        <>
            <PageHeader
                description="Guild-wide Quack configuration and module state."
                title="Settings"
            />
            {settings.isPending ||
            appealSettings.isPending ||
            guildContext.isPending ||
            session.isPending ? (
                <QueryPending label="Loading settings" />
            ) : settings.isError ||
              appealSettings.isError ||
              guildContext.isError ||
              session.isError ? (
                <QueryError
                    onRetry={() => {
                        settings.refetch();
                        appealSettings.refetch();
                        guildContext.refetch();
                        session.refetch();
                    }}
                />
            ) : guildContext.data.permissions["guild_settings.write"] ? (
                <>
                    {settings.data.starter_policy_review_required &&
                    !settings.data.starter_policy_notice_acknowledged_at ? (
                        <StarterPolicyNotice
                            csrfToken={session.data.csrf_token}
                            guildId={guildId}
                        />
                    ) : null}
                    <SettingsForm
                        csrfToken={session.data.csrf_token}
                        guildId={guildId}
                        settings={settings.data}
                    />
                    <AppealSettingsForm
                        csrfToken={session.data.csrf_token}
                        guildId={guildId}
                        settings={appealSettings.data}
                    />
                </>
            ) : (
                <Alert>
                    <AlertTitle>Read-only access</AlertTitle>
                    <AlertDescription>
                        Your current Discord permissions do not allow changing
                        guild settings.
                    </AlertDescription>
                </Alert>
            )}
        </>
    );
}
