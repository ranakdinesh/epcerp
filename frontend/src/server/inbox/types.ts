export type Notice = {
    source_action_id?: string; notice_kind?: string; title?: string; link?: string;
    id: string; source_event_id: string; recipient_id: string;
    director_designation_id: string; director_version: number; decision_id: string;
    project_id: string; source_version: number; decision_action: string;
    version: number; is_active: boolean; read_at: string | null; acknowledged_at: string | null;
    created_at: string; updated_at: string; created_by_id: string | null; updated_by_id: string | null;
};
export type History = { id: string; action: string; before: Notice | null; after: Notice; created_by_id: string; created_at: string };
