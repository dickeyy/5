import { useQuery } from "@tanstack/react-query";
import { AlertCircleIcon, BirdIcon } from "lucide-react";
import type { ReactNode } from "react";
import { Alert, AlertDescription, AlertTitle } from "#/components/ui/alert";
import { Button } from "#/components/ui/button";
import { Spinner } from "#/components/ui/spinner";
import { ApiError, apiUrl } from "#/lib/api";
import { sessionQuery } from "#/lib/queries";

export function AuthGate({ children }: { children: ReactNode }) {
    const session = useQuery(sessionQuery);

    if (session.isPending) {
        return (
            <main className="flex min-h-svh items-center justify-center">
                <Spinner />
                <span className="sr-only">Checking your session</span>
            </main>
        );
    }

    if (session.error instanceof ApiError && session.error.status === 401) {
        return <SignIn />;
    }

    if (session.isError) {
        return (
            <main className="mx-auto flex min-h-svh w-full max-w-lg items-center px-6">
                <Alert variant="error">
                    <AlertCircleIcon aria-hidden="true" />
                    <AlertTitle>Could not reach Quack</AlertTitle>
                    <AlertDescription>
                        <p>
                            The dashboard could not verify your session. Check
                            your connection and try again.
                        </p>
                        <div>
                            <Button
                                onClick={() => session.refetch()}
                                type="button"
                                variant="outline"
                            >
                                Try again
                            </Button>
                        </div>
                    </AlertDescription>
                </Alert>
            </main>
        );
    }

    return children;
}

function SignIn() {
    const redirectTo =
        typeof window === "undefined" ? "/" : window.location.href;
    const signInUrl = apiUrl(
        `/auth/discord/login?redirect_to=${encodeURIComponent(redirectTo)}`,
    );

    return (
        <main className="flex min-h-svh items-center justify-center px-6">
            <div className="flex w-full max-w-sm flex-col items-center gap-6 text-center">
                <div className="flex size-11 items-center justify-center rounded-xl border bg-card shadow-xs/5">
                    <BirdIcon aria-hidden="true" />
                </div>
                <div className="flex flex-col gap-2">
                    <h1 className="font-heading font-semibold text-2xl">
                        Sign in to Quack
                    </h1>
                    <p className="text-muted-foreground text-sm">
                        Use your Discord account to access the servers you
                        moderate.
                    </p>
                </div>
                <Button
                    render={<a href={signInUrl}>Continue with Discord</a>}
                    size="lg"
                />
            </div>
        </main>
    );
}
