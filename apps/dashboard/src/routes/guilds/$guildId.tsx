import {
    createFileRoute,
    Outlet,
    useRouterState,
} from "@tanstack/react-router";
import { AppShell } from "#/components/dashboard/app-shell";

export const Route = createFileRoute("/guilds/$guildId")({
    component: GuildRouteLayout,
});

function GuildRouteLayout() {
    const isMemberAppealRoute = useRouterState({
        select: (state) =>
            state.location.pathname.replace(/\/+$/, "").endsWith("/appeal"),
    });

    return isMemberAppealRoute ? <Outlet /> : <AppShell />;
}
