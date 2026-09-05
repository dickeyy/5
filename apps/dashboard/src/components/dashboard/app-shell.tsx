import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
    Link,
    Outlet,
    useParams,
    useRouterState,
} from "@tanstack/react-router";
import {
    BirdIcon,
    BookOpenIcon,
    ChartNoAxesColumnIncreasingIcon,
    ChevronUpIcon,
    ClipboardListIcon,
    FileClockIcon,
    LogOutIcon,
    ScrollTextIcon,
    SettingsIcon,
    ShieldAlertIcon,
    TriangleAlertIcon,
} from "lucide-react";
import { Avatar, AvatarFallback, AvatarImage } from "#/components/ui/avatar";
import {
    Menu,
    MenuItem,
    MenuPopup,
    MenuSeparator,
    MenuTrigger,
} from "#/components/ui/menu";
import {
    Sidebar,
    SidebarContent,
    SidebarFooter,
    SidebarGroup,
    SidebarGroupContent,
    SidebarGroupLabel,
    SidebarHeader,
    SidebarInset,
    SidebarMenu,
    SidebarMenuButton,
    SidebarMenuItem,
    SidebarProvider,
    SidebarRail,
    SidebarTrigger,
} from "#/components/ui/sidebar";
import { logout } from "#/lib/api";
import { guildContextQuery, sessionQuery } from "#/lib/queries";
import { QueryError, QueryPending } from "./query-state";

const navigation = [
    {
        label: "Overview",
        path: "/guilds/$guildId",
        icon: ChartNoAxesColumnIncreasingIcon,
    },
    {
        label: "Cases",
        path: "/guilds/$guildId/cases",
        icon: ClipboardListIcon,
        permission: "case.read",
    },
    {
        label: "Templates",
        path: "/guilds/$guildId/templates",
        icon: BookOpenIcon,
        permission: "case_template.read",
    },
    {
        label: "Appeals",
        path: "/guilds/$guildId/appeals",
        icon: ShieldAlertIcon,
        permission: "appeal.review",
    },
    {
        label: "Action failures",
        path: "/guilds/$guildId/action-failures",
        icon: TriangleAlertIcon,
        permission: "case.read",
    },
    {
        label: "Audit log",
        path: "/guilds/$guildId/audit-log",
        icon: ScrollTextIcon,
        permission: "audit.read",
    },
] as const;

export function AppShell() {
    const { guildId } = useParams({ from: "/guilds/$guildId" });
    const pathName = useRouterState({
        select: (state) => state.location.pathname,
    });
    const guildContext = useQuery(guildContextQuery(guildId));
    const session = useQuery(sessionQuery);
    const queryClient = useQueryClient();
    const logoutMutation = useMutation({
        mutationFn: async () => {
            if (!session.data) {
                return;
            }
            await logout(session.data.csrf_token);
        },
        onSuccess: () => {
            queryClient.removeQueries();
            window.location.assign("/");
        },
    });

    if (guildContext.isPending || session.isPending) {
        return <QueryPending label="Loading guild" />;
    }

    if (guildContext.isError || session.isError) {
        return (
            <main className="mx-auto flex min-h-svh w-full max-w-2xl items-center px-6">
                <QueryError
                    description="You may no longer have access to this server, or Discord could not verify your permissions."
                    onRetry={() => {
                        guildContext.refetch();
                        session.refetch();
                    }}
                />
            </main>
        );
    }

    const user = session.data.user;
    const guild = guildContext.data.guild;
    const canReadSettings = Boolean(
        guildContext.data.permissions["guild_settings.read"],
    );

    return (
        <SidebarProvider>
            <Sidebar collapsible="icon">
                <SidebarHeader>
                    <SidebarMenu>
                        <SidebarMenuItem>
                            <SidebarMenuButton
                                render={<Link to="/" />}
                                size="lg"
                                tooltip="Change server"
                            >
                                <Avatar className="size-8 rounded-lg">
                                    <AvatarImage
                                        alt=""
                                        src={guild.icon_url || undefined}
                                    />
                                    <AvatarFallback className="rounded-lg">
                                        {guild.name.slice(0, 2).toUpperCase()}
                                    </AvatarFallback>
                                </Avatar>
                                <span className="flex min-w-0 flex-col">
                                    <span className="truncate font-medium text-sidebar-accent-foreground">
                                        {guild.name}
                                    </span>
                                    <span className="truncate text-xs">
                                        Change server
                                    </span>
                                </span>
                            </SidebarMenuButton>
                        </SidebarMenuItem>
                    </SidebarMenu>
                </SidebarHeader>
                <SidebarContent>
                    <SidebarGroup>
                        <SidebarGroupLabel>Moderation</SidebarGroupLabel>
                        <SidebarGroupContent>
                            <SidebarMenu>
                                {navigation
                                    .filter(
                                        (item) =>
                                            !("permission" in item) ||
                                            guildContext.data.permissions[
                                                item.permission
                                            ],
                                    )
                                    .map((item) => (
                                        <SidebarMenuItem key={item.path}>
                                            <SidebarMenuButton
                                                isActive={isActivePath(
                                                    pathName,
                                                    item.path,
                                                    guildId,
                                                )}
                                                render={
                                                    <Link
                                                        params={{ guildId }}
                                                        to={item.path}
                                                    />
                                                }
                                                tooltip={item.label}
                                            >
                                                <item.icon aria-hidden="true" />
                                                <span>{item.label}</span>
                                            </SidebarMenuButton>
                                        </SidebarMenuItem>
                                    ))}
                            </SidebarMenu>
                        </SidebarGroupContent>
                    </SidebarGroup>
                    {canReadSettings ? (
                        <SidebarGroup className="mt-auto">
                            <SidebarGroupLabel>
                                Administration
                            </SidebarGroupLabel>
                            <SidebarGroupContent>
                                <SidebarMenu>
                                    <SidebarMenuItem>
                                        <SidebarMenuButton
                                            isActive={pathName.endsWith(
                                                "/settings",
                                            )}
                                            render={
                                                <Link
                                                    params={{ guildId }}
                                                    to="/guilds/$guildId/settings"
                                                />
                                            }
                                            tooltip="Settings"
                                        >
                                            <SettingsIcon aria-hidden="true" />
                                            <span>Settings</span>
                                        </SidebarMenuButton>
                                    </SidebarMenuItem>
                                </SidebarMenu>
                            </SidebarGroupContent>
                        </SidebarGroup>
                    ) : null}
                </SidebarContent>
                <SidebarFooter>
                    <SidebarMenu>
                        <SidebarMenuItem>
                            <Menu>
                                <MenuTrigger
                                    render={<SidebarMenuButton size="lg" />}
                                >
                                    <Avatar className="size-8 rounded-lg">
                                        <AvatarImage
                                            alt=""
                                            src={user.avatar_url || undefined}
                                        />
                                        <AvatarFallback className="rounded-lg">
                                            {(user.global_name || user.username)
                                                .slice(0, 2)
                                                .toUpperCase()}
                                        </AvatarFallback>
                                    </Avatar>
                                    <span className="flex min-w-0 flex-1 flex-col">
                                        <span className="truncate font-medium text-sidebar-accent-foreground">
                                            {user.global_name || user.username}
                                        </span>
                                        <span className="truncate text-xs">
                                            @{user.username}
                                        </span>
                                    </span>
                                    <ChevronUpIcon aria-hidden="true" />
                                </MenuTrigger>
                                <MenuPopup
                                    align="start"
                                    className="min-w-56"
                                    side="top"
                                    sideOffset={8}
                                >
                                    <MenuItem
                                        render={<Link to="/" />}
                                        closeOnClick
                                    >
                                        <BirdIcon aria-hidden="true" />
                                        Change server
                                    </MenuItem>
                                    <MenuSeparator />
                                    <MenuItem
                                        closeOnClick
                                        disabled={logoutMutation.isPending}
                                        onClick={() => logoutMutation.mutate()}
                                    >
                                        <LogOutIcon aria-hidden="true" />
                                        Sign out
                                    </MenuItem>
                                </MenuPopup>
                            </Menu>
                        </SidebarMenuItem>
                    </SidebarMenu>
                </SidebarFooter>
                <SidebarRail />
            </Sidebar>
            <SidebarInset>
                <div className="flex h-14 items-center gap-2 border-b px-4">
                    <SidebarTrigger />
                    <div className="flex items-center gap-2 text-sm">
                        <FileClockIcon
                            aria-hidden="true"
                            className="text-muted-foreground"
                        />
                        <span className="font-medium">{guild.name}</span>
                    </div>
                </div>
                <div className="mx-auto flex w-full max-w-7xl flex-1 flex-col gap-8 p-4 sm:p-6 lg:p-8">
                    <Outlet />
                </div>
            </SidebarInset>
        </SidebarProvider>
    );
}

function isActivePath(
    currentPath: string,
    routePath: string,
    guildId: string,
): boolean {
    const concretePath = routePath.replace("$guildId", guildId);
    if (routePath === "/guilds/$guildId") {
        return (
            currentPath === concretePath || currentPath === `${concretePath}/`
        );
    }
    return currentPath.startsWith(concretePath);
}
