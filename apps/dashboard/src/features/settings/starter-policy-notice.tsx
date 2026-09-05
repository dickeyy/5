import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
    Alert,
    AlertAction,
    AlertDescription,
    AlertTitle,
} from "#/components/ui/alert";
import { Button } from "#/components/ui/button";
import { acknowledgeStarterPolicyNotice } from "#/lib/api";

export function StarterPolicyNotice({
    csrfToken,
    guildId,
}: {
    csrfToken: string;
    guildId: string;
}) {
    const queryClient = useQueryClient();
    const mutation = useMutation({
        mutationFn: () => acknowledgeStarterPolicyNotice(guildId, csrfToken),
        onSuccess: async () => {
            await queryClient.invalidateQueries({
                queryKey: ["guild", guildId, "settings"],
            });
        },
    });

    return (
        <Alert variant={mutation.isError ? "error" : "warning"}>
            <AlertTitle>
                {mutation.isError
                    ? "Notice was not acknowledged"
                    : "Review the starter policy"}
            </AlertTitle>
            <AlertDescription>
                {mutation.isError
                    ? mutation.error.message
                    : "Quack created a starter moderation template. Review its context, escalation, and notification behavior before relying on it."}
            </AlertDescription>
            <AlertAction>
                <Button
                    loading={mutation.isPending}
                    onClick={() => mutation.mutate()}
                    size="sm"
                    variant="outline"
                >
                    I understand
                </Button>
            </AlertAction>
        </Alert>
    );
}
