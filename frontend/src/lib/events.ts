export type EventSummary = {
  id: string
  creator_id: string
  title: string
  description?: string
  category?: string
  event_date: string
  event_date_label?: string
  start_time?: string
  end_time?: string
  location_name?: string
  lat?: number
  lng?: number
  max_participants?: number
  visibility: string
  status?: string
  live_location_enabled?: boolean
  member_count: number
}

export type JoinRequest = {
  id: string
  user_id: string
  username: string
  full_name?: string
  message?: string
}

export type EventHost = {
  user_id: string
  username: string
  full_name?: string
  role: string
  is_creator: boolean
}

export type EventDetail = {
  event: EventSummary
  is_creator: boolean
  is_host: boolean
  is_member: boolean
  has_pending_join: boolean
  my_invitation_id?: string
  pending_join_requests: JoinRequest[]
  hosts: EventHost[]
  accepted_invites?: number
  pending_invites?: number
  total_invites?: number
}
