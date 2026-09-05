import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CheckIcon, RotateCcwIcon } from "lucide-react";
import { Button } from "#/components/ui/button";
import { dismissActionFailure, retryActionFailure } from "#/lib/api";
import type { FailedAction } from "#/lib/api-types";

type FailureActionsProps = {
    action: FailedAction;
    canDismiss: boolean;
    canRetry: boolean;
    csrfToken: string;
    guildId: string;
};

export function FailureActions({
    action,
    canDismiss,
    canRetry,
    csrfToken,
    guildId,
}: FailureActionsProps) {
    const queryClient = useQueryClient();
    const retry = useMutation({
        mutationFn: () => retryActionFailure(guildId, action.id, csrfToken),
        onSuccess: invalidate,
    });
    const dismiss = useMutation({
        mutationFn: () => dismissActionFailure(guildId, action.id, csrfToken),
        onSuccess: invalidate,
    });

    async function invalidate() {
        await Promise.all([
            queryClient.invalidateQueries({
                queryKey: ["guild", guildId, "action-failures"],
            }),
            queryClient.invalidateQueries({
                queryKey: ["guild", guildId, "cases"],
            }),
            queryClient.invalidateQueries({
                queryKey: ["guild", guildId, "statistics"],
            }),
        ]);
    }

    const error = retry.error ?? dismiss.error;

    return (
        <div className="flex flex-col items-end gap-1">
            <div className="flex justify-end gap-2">
                {canRetry && action.safe_for_retry ? (
                    <Button
                        loading={retry.isPending}
                        onClick={() => retry.mutate()}
                        size="sm"
                        variant="outline"
                    >
                        <RotateCcwIcon aria-hidden="true" />
                        Retry
                    </Button>
                ) : null}
                {canDismiss ? (
                    <Button
                        loading={dismiss.isPending}
                        onClick={() => dismiss.mutate()}
                        size="sm"
                        variant="ghost"
                    >
                        <CheckIcon aria-hidden="true" />
                        Dismiss
                    </Button>
                ) : null}
            </div>
            {error ? (
                <span className="text-destructive text-xs" role="alert">
                    {error.message}
                </span>
            ) : null}
        </div>
    );
}
