import type { ErrorComponentProps } from "@tanstack/react-router";
import { AlertCircleIcon } from "lucide-react";
import { Alert, AlertDescription, AlertTitle } from "#/components/ui/alert";
import { Button } from "#/components/ui/button";

// RootError gives unexpected route failures a recoverable, user-facing
// boundary instead of falling through to TanStack Router's development warning.
export function RootError({ reset }: ErrorComponentProps) {
    return (
        <main className="mx-auto flex min-h-svh w-full max-w-lg items-center px-6">
            <Alert variant="error">
                <AlertCircleIcon aria-hidden="true" />
                <AlertTitle>The dashboard encountered an error</AlertTitle>
                <AlertDescription>
                    <p>
                        Try the page again. If the problem continues, reload the
                        dashboard.
                    </p>
                    <div className="flex flex-wrap gap-2">
                        <Button onClick={reset} type="button">
                            Try again
                        </Button>
                        <Button
                            onClick={() => window.location.reload()}
                            type="button"
                            variant="outline"
                        >
                            Reload dashboard
                        </Button>
                    </div>
                </AlertDescription>
            </Alert>
        </main>
    );
}
