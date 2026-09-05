import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useId, useState } from "react";
import { Alert, AlertDescription, AlertTitle } from "#/components/ui/alert";
import { Button } from "#/components/ui/button";
import { Card, CardHeader, CardPanel, CardTitle } from "#/components/ui/card";
import { Field, FieldDescription, FieldLabel } from "#/components/ui/field";
import { Input } from "#/components/ui/input";
import { Separator } from "#/components/ui/separator";
import { Switch } from "#/components/ui/switch";
import { Textarea } from "#/components/ui/textarea";
import { updateGuildSettings } from "#/lib/api";
import type { GuildSettings, GuildSettingsInput } from "#/lib/api-types";

type SettingsFormProps = {
    csrfToken: string;
    guildId: string;
    settings: GuildSettings;
};

export function SettingsForm({
    csrfToken,
    guildId,
    settings,
}: SettingsFormProps) {
    const queryClient = useQueryClient();
    const [ticketsEnabled, setTicketsEnabled] = useState(
        settings.tickets_enabled,
    );
    const [loggingEnabled, setLoggingEnabled] = useState(
        settings.general_logging_enabled,
    );
    const [honeypotEnabled, setHoneypotEnabled] = useState(
        settings.honeypot_enabled,
    );
    const [saved, setSaved] = useState(false);
    const mutation = useMutation({
        mutationFn: (input: GuildSettingsInput) =>
            updateGuildSettings(guildId, input, csrfToken),
        onMutate: () => setSaved(false),
        onSuccess: async () => {
            await queryClient.invalidateQueries({
                queryKey: ["guild", guildId, "settings"],
            });
            setSaved(true);
        },
    });

    function submit(event: React.FormEvent<HTMLFormElement>) {
        event.preventDefault();
        const formData = new FormData(event.currentTarget);
        mutation.mutate({
            audit_mirror_channel_discord_id: String(
                formData.get("audit_mirror_channel_discord_id") ?? "",
            ),
            general_logging_enabled: loggingEnabled,
            honeypot_enabled: honeypotEnabled,
            managed_evidence_channel_discord_id: String(
                formData.get("managed_evidence_channel_discord_id") ?? "",
            ),
            notification_footer: String(
                formData.get("notification_footer") ?? "",
            ),
            notification_introduction: String(
                formData.get("notification_introduction") ?? "",
            ),
            tickets_enabled: ticketsEnabled,
        });
    }

    return (
        <form className="flex flex-col gap-4" onSubmit={submit}>
            <div className="grid gap-4 lg:grid-cols-2">
                <Card>
                    <CardHeader>
                        <CardTitle>Managed channels</CardTitle>
                    </CardHeader>
                    <CardPanel className="flex flex-col gap-5">
                        <Field name="managed_evidence_channel_discord_id">
                            <FieldLabel>Evidence channel ID</FieldLabel>
                            <Input
                                defaultValue={
                                    settings.managed_evidence_channel_discord_id ??
                                    ""
                                }
                                inputMode="numeric"
                                maxLength={20}
                                name="managed_evidence_channel_discord_id"
                                pattern="[0-9]*"
                                placeholder="Not configured"
                                type="text"
                            />
                            <FieldDescription>
                                Staff-only channel used to preserve case
                                evidence.
                            </FieldDescription>
                        </Field>
                        <Field name="audit_mirror_channel_discord_id">
                            <FieldLabel>Audit mirror channel ID</FieldLabel>
                            <Input
                                defaultValue={
                                    settings.audit_mirror_channel_discord_id ??
                                    ""
                                }
                                inputMode="numeric"
                                maxLength={20}
                                name="audit_mirror_channel_discord_id"
                                pattern="[0-9]*"
                                placeholder="Not configured"
                                type="text"
                            />
                            <FieldDescription>
                                Optional Discord channel for important audit
                                events.
                            </FieldDescription>
                        </Field>
                    </CardPanel>
                </Card>
                <Card>
                    <CardHeader>
                        <CardTitle>Member notifications</CardTitle>
                    </CardHeader>
                    <CardPanel className="flex flex-col gap-5">
                        <Field name="notification_introduction">
                            <FieldLabel>Introduction</FieldLabel>
                            <Textarea
                                defaultValue={
                                    settings.notification_introduction ?? ""
                                }
                                maxLength={2000}
                                name="notification_introduction"
                                placeholder="Optional text shown before the case details."
                            />
                        </Field>
                        <Field name="notification_footer">
                            <FieldLabel>Footer</FieldLabel>
                            <Textarea
                                defaultValue={
                                    settings.notification_footer ?? ""
                                }
                                maxLength={2000}
                                name="notification_footer"
                                placeholder="Optional text shown after the case details."
                            />
                        </Field>
                    </CardPanel>
                </Card>
            </div>
            <Card>
                <CardHeader>
                    <CardTitle>Optional modules</CardTitle>
                </CardHeader>
                <CardPanel className="flex flex-col">
                    <ModuleSwitch
                        checked={ticketsEnabled}
                        description="Private Discord support threads between members and staff."
                        label="Tickets"
                        onCheckedChange={setTicketsEnabled}
                    />
                    <Separator />
                    <ModuleSwitch
                        checked={loggingEnabled}
                        description="Selected Discord events sent to configured staff channels."
                        label="General logging"
                        onCheckedChange={setLoggingEnabled}
                    />
                    <Separator />
                    <ModuleSwitch
                        checked={honeypotEnabled}
                        description="Configured traps that apply a selected case template."
                        label="Honeypots"
                        onCheckedChange={setHoneypotEnabled}
                    />
                </CardPanel>
            </Card>
            {mutation.isError ? (
                <Alert variant="error">
                    <AlertTitle>Settings were not saved</AlertTitle>
                    <AlertDescription>
                        {mutation.error.message}
                    </AlertDescription>
                </Alert>
            ) : null}
            {saved ? (
                <Alert variant="success">
                    <AlertTitle>Settings saved</AlertTitle>
                    <AlertDescription>
                        The guild configuration is up to date.
                    </AlertDescription>
                </Alert>
            ) : null}
            <div className="flex justify-end">
                <Button loading={mutation.isPending} type="submit">
                    Save settings
                </Button>
            </div>
        </form>
    );
}

function ModuleSwitch({
    checked,
    description,
    label,
    onCheckedChange,
}: {
    checked: boolean;
    description: string;
    label: string;
    onCheckedChange: (checked: boolean) => void;
}) {
    const id = useId();

    return (
        <div className="flex items-center justify-between gap-4 py-4">
            <div className="flex flex-col gap-1">
                <FieldLabel htmlFor={id}>{label}</FieldLabel>
                <p className="text-muted-foreground text-xs">{description}</p>
            </div>
            <Switch
                checked={checked}
                id={id}
                onCheckedChange={onCheckedChange}
            />
        </div>
    );
}
