export type Session = {
    csrf_token: string;
    user: {
        id: string;
        username: string;
        global_name: string;
        avatar: string;
        avatar_url: string;
    };
    session: {
        expires_at: string;
        last_seen: string;
    };
};

export type GuildListItem = {
    discord_guild_id: string;
    name: string;
    icon_url: string;
    permission_bits: string;
    is_owner: boolean;
    is_administrator: boolean;
    can_manage_guild: boolean;
    can_moderate: boolean;
    quack_in_guild: boolean;
    quack_guild_name?: string;
};

export type GuildContext = {
    guild: {
        id: string;
        discord_guild_id: string;
        name: string;
        icon_url: string;
        owner_discord_user_id: string;
    };
    staff: {
        id: string;
        discord_user_id: string;
        display_name: string;
        permission_bits: string;
        is_admin: boolean;
        is_moderator: boolean;
        last_active_at: string;
        last_seen_permissions: string;
    };
    permissions: Record<string, boolean>;
};

export type StatisticBucket = {
    key: string;
    count: number;
};

export type StaffStatistics = {
    from: string;
    to: string;
    case_total: number;
    action_total: number;
    appeal_total: number;
    audit_total: number;
    cases_by_day: StatisticBucket[];
    cases_by_template: StatisticBucket[];
    cases_by_validity: StatisticBucket[];
    cases_by_source: StatisticBucket[];
    actions_by_day: StatisticBucket[];
    actions_by_type: StatisticBucket[];
    actions_by_result: StatisticBucket[];
    appeals_by_day: StatisticBucket[];
    appeals_by_status: StatisticBucket[];
    audits_by_day: StatisticBucket[];
    audits_by_action: StatisticBucket[];
    audits_by_result: StatisticBucket[];
    audits_by_source: StatisticBucket[];
};

export type CaseAction = {
    id: string;
    position: number;
    action_type: string;
    status: string;
    attempt_count?: number;
    last_error_code?: string;
    last_error?: string;
    started_at?: string;
    finished_at?: string;
    next_retry_at?: string;
    safe_for_retry: boolean;
    irreversible: boolean;
    attempts?: Array<{
        id: string;
        attempt_number: number;
        status: string;
        started_at: string;
        finished_at?: string;
        duration_ms: number;
        error_code?: string;
        error_message?: string;
    }>;
};

export type SelectedLevel = {
    id: string;
    name: string;
    position: number;
    is_default: boolean;
    trigger_case_count: number;
    notify_user: boolean;
    matched_case_count: number;
};

export type Case = {
    id: string;
    guild_id: string;
    case_number: number;
    template_id: string | null;
    template_version: number;
    target_discord_user_id: string;
    moderator_discord_user_id: string;
    reason: string;
    validity: string;
    source: string;
    context_channel_discord_id?: string;
    context_message_discord_id?: string;
    context_url?: string;
    context_values?: Array<{
        key: string;
        label: string;
        required: boolean;
        type: string;
        value: unknown;
    }>;
    voided_reason?: string;
    voided_at?: string;
    replacement_case_id?: string;
    replaces_case_id?: string;
    created_at: string;
    updated_at: string;
    selected_level?: SelectedLevel;
    actions: CaseAction[];
};

export type CaseDetail = Case & {
    events: Array<{
        id: string;
        created_at: string;
        event_type: string;
        actor_discord_user_id?: string;
        actor_type: string;
        visibility: string;
        body: string;
    }>;
    evidence: Array<{
        id: string;
        author_discord_user_id: string;
        message_url: string;
        content: string;
        capture_outcome: string;
        capture_warning?: string;
        message_created_at: string;
        attachments: Array<{
            filename: string;
            content_type: string;
            original_url: string;
            preserved_url?: string;
            copy_outcome: string;
            warning?: string;
            size_bytes: number;
        }>;
    }>;
    notification?: {
        status: string;
        attempt_count: number;
        last_error_code?: string;
        last_error?: string;
        sent_at?: string;
    };
};

export type MemberCase = {
    id: string;
    guild_id: string;
    case_number: number;
    template_id: string | null;
    official_reason: string;
    validity: string;
    voided_reason?: string;
    replacement_case_id?: string;
    created_at: string;
    context: NonNullable<Case["context_values"]>;
    selected_outcome?: SelectedLevel;
    enforcement?: {
        action_type: string;
        status: string;
    };
    evidence: CaseDetail["evidence"];
    history: CaseDetail["events"];
    notification?: CaseDetail["notification"];
    appealable: boolean;
    appeal_id?: string;
    appeal_status?: string;
};

export type MemberCaseSummary = {
    id: string;
    guild_id: string;
    case_number: number;
    official_reason: string;
    validity: string;
    created_at: string;
    selected_outcome?: SelectedLevel;
    appealable: boolean;
    appeal_id?: string;
    appeal_status?: string;
};

export type MemberCaseList = {
    cases: MemberCaseSummary[];
    total: number;
    limit: number;
    offset: number;
};

export type CaseList = {
    cases: Case[];
    total: number;
    limit: number;
    offset: number;
};

export type CaseProfile = CaseList & {
    summary: {
        total: number;
        by_validity: Record<string, number>;
        by_template: Record<string, number>;
    };
};

export type FailedAction = {
    id: string;
    case_id: string;
    action_type: string;
    status: string;
    attempt_count: number;
    max_retries: number;
    safe_for_retry: boolean;
    last_error_code?: string;
    last_error?: string;
    created_at: string;
    updated_at: string;
};

export type FailedActionList = {
    executions: FailedAction[];
    total: number;
};

export type CaseCreateInput = {
    template_id: string;
    target_discord_user_id: string;
    context_values: Array<{
        key: string;
        value: unknown;
    }>;
    context_channel_discord_id?: string;
    context_message_discord_id?: string;
    context_url?: string;
    evidence_links?: string[];
    metadata?: Record<string, unknown>;
    replaces_case_id?: string;
};

export type TemplateContextField = {
    id: string;
    key: string;
    label: string;
    type: string;
    position: number;
    required: boolean;
};

export type TemplateLevel = {
    id: string;
    name: string;
    position: number;
    is_default: boolean;
    trigger_case_count: number;
    notify_user: boolean;
    actions: Array<{
        id: string;
        action_type: string;
        timeout_duration_seconds?: number;
        delete_message_seconds?: number;
        max_retries: number;
    }>;
};

export type Template = {
    id: string;
    guild_id: string;
    slug: string;
    name: string;
    description: string;
    reason_template: string;
    appealable: boolean;
    version: number;
    archived_at: string | null;
    context_fields: TemplateContextField[];
    levels: TemplateLevel[];
};

export type TemplateContextFieldInput = {
    key: string;
    label: string;
    type: string;
    position: number;
    required: boolean;
};

export type TemplateActionInput = {
    action_type: string;
    timeout_duration_seconds?: number;
    delete_message_seconds?: number;
    max_retries: number;
};

export type TemplateLevelInput = {
    name: string;
    position: number;
    is_default: boolean;
    trigger_case_count: number;
    notify_user: boolean;
    actions: TemplateActionInput[];
};

export type TemplateInput = {
    slug: string;
    name: string;
    description: string;
    reason_template: string;
    appealable: boolean;
    context_fields: TemplateContextFieldInput[];
    levels: TemplateLevelInput[];
};

export type TemplatePolicy = {
    schema_version: number;
    slug: string;
    name: string;
    description: string;
    official_reason: string;
    appealable: boolean;
    context_fields: TemplateContextFieldInput[];
    levels: TemplateLevelInput[];
};

export type Appeal = {
    id: string;
    guild_id: string;
    case_id: string;
    target_discord_user_id: string;
    status: string;
    questions: Array<{
        id: string;
        prompt: string;
        type: string;
        required: boolean;
        position: number;
    }>;
    answers: Array<{
        question_id: string;
        value: unknown;
    }>;
    decision_reason?: string;
    reviewed_by_discord_user_id?: string;
    events: Array<{
        id: string;
        type: string;
        actor_type: string;
        actor_discord_user_id?: string;
        body: string;
        created_at: string;
    }>;
    reversal_offers?: Array<{
        original_execution_id: string;
        action_type: string;
    }>;
    created_at: string;
    updated_at: string;
};

export type AppealTransition =
    | "accept"
    | "reject"
    | "close"
    | "reopen"
    | "request-information";

export type AppealList = {
    appeals: Appeal[];
    total: number;
    limit: number;
    offset: number;
};

export type AppealQuestion = {
    id: string;
    prompt: string;
    type: "short_text" | "long_text" | "boolean";
    required: boolean;
    position: number;
};

export type AppealSettings = {
    guild_id: string;
    questions: AppealQuestion[];
    default: boolean;
};

export type AuditEntry = {
    id: string;
    created_at: string;
    actor_discord_user_id?: string;
    source: string;
    action: string;
    resource_type: string;
    resource_id: string;
    result: string;
    failure_reason?: string;
};

export type AuditList = {
    entries: AuditEntry[];
    total: number;
    limit: number;
    offset: number;
    next_cursor?: string;
};

export type GuildSettings = {
    id: string;
    guild_id: string;
    audit_mirror_channel_discord_id?: string;
    managed_evidence_channel_discord_id?: string;
    notification_introduction?: string;
    notification_footer?: string;
    tickets_enabled: boolean;
    general_logging_enabled: boolean;
    honeypot_enabled: boolean;
    starter_policy_template_id: string;
    starter_policy_review_required: boolean;
    starter_policy_notice_acknowledged_at?: string;
};

export type GuildSettingsInput = {
    audit_mirror_channel_discord_id?: string;
    managed_evidence_channel_discord_id?: string;
    notification_introduction?: string;
    notification_footer?: string;
    tickets_enabled?: boolean;
    general_logging_enabled?: boolean;
    honeypot_enabled?: boolean;
};
