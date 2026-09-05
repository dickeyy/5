import { QueryClient } from "@tanstack/react-query";

export function getContext() {
    const queryClient = new QueryClient({
        defaultOptions: {
            queries: {
                refetchOnWindowFocus: false,
                retry: 1,
                staleTime: 15_000,
            },
        },
    });

    return {
        queryClient,
    };
}
export default function TanstackQueryProvider() {}
