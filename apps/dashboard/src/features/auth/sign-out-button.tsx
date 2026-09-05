import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { LogOutIcon } from "lucide-react";
import { Button } from "#/components/ui/button";
import { logout } from "#/lib/api";
import { sessionQuery } from "#/lib/queries";

export function SignOutButton() {
    const queryClient = useQueryClient();
    const session = useQuery(sessionQuery);
    const mutation = useMutation({
        mutationFn: () =>
            session.data ? logout(session.data.csrf_token) : Promise.resolve(),
        onSuccess: () => {
            queryClient.removeQueries();
            window.location.assign("/");
        },
    });

    if (!session.data) {
        return null;
    }

    return (
        <div className="flex flex-col items-end gap-1">
            <Button
                loading={mutation.isPending}
                onClick={() => mutation.mutate()}
                size="sm"
                variant="ghost"
            >
                <LogOutIcon aria-hidden="true" />
                Sign out
            </Button>
            {mutation.isError ? (
                <span className="text-destructive text-xs" role="alert">
                    {mutation.error.message}
                </span>
            ) : null}
        </div>
    );
}
