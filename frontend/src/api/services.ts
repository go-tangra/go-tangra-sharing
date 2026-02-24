/**
 * Sharing Module Service Functions
 *
 * Typed service methods for the Sharing API using dynamic module routing.
 * Base URL: /admin/v1/modules/sharing/v1
 */

import { sharingApi, type RequestOptions } from './client';

// ==================== Entity Types ====================

export interface SharedLink {
  id: string;
  tenantId: number;
  resourceType: 'RESOURCE_TYPE_SECRET' | 'RESOURCE_TYPE_DOCUMENT';
  resourceId: string;
  resourceName: string;
  token: string;
  recipientEmail: string;
  message: string;
  viewed: boolean;
  viewedAt?: string;
  revoked: boolean;
  createdBy?: number;
  createTime: string;
  policies?: SharePolicy[];
}

export interface EmailTemplate {
  id: string;
  tenantId: number;
  name: string;
  subject: string;
  htmlBody: string;
  isDefault: boolean;
  createdBy?: number;
  updatedBy?: number;
  createTime: string;
  updateTime?: string;
}

export type SharePolicyType =
  | 'SHARE_POLICY_TYPE_BLACKLIST'
  | 'SHARE_POLICY_TYPE_WHITELIST';

export type SharePolicyMethod =
  | 'SHARE_POLICY_METHOD_IP'
  | 'SHARE_POLICY_METHOD_MAC'
  | 'SHARE_POLICY_METHOD_REGION'
  | 'SHARE_POLICY_METHOD_TIME'
  | 'SHARE_POLICY_METHOD_DEVICE'
  | 'SHARE_POLICY_METHOD_NETWORK';

export interface SharePolicy {
  id: string;
  shareLinkId: string;
  type: SharePolicyType;
  method: SharePolicyMethod;
  value: string;
  reason: string;
  createTime: string;
}

export interface CreateSharePolicyRequest {
  type: SharePolicyType;
  method: SharePolicyMethod;
  value: string;
  reason?: string;
}

export interface CreateSharePolicyInput {
  type: SharePolicyType;
  method: SharePolicyMethod;
  value: string;
  reason?: string;
}

// ==================== Request/Response Types ====================

export interface CreateShareRequest {
  resourceType: 'RESOURCE_TYPE_SECRET' | 'RESOURCE_TYPE_DOCUMENT';
  resourceId: string;
  recipientEmail: string;
  message?: string;
  templateId?: string;
  policies?: CreateSharePolicyInput[];
}

export interface CreateShareResponse {
  shareId: string;
  shareLink: string;
}

export interface ListSharesResponse {
  shares: SharedLink[];
  total: number;
}

export interface CreateTemplateRequest {
  name: string;
  subject: string;
  htmlBody: string;
  isDefault?: boolean;
}

export interface UpdateTemplateRequest {
  name?: string;
  subject?: string;
  htmlBody?: string;
  isDefault?: boolean;
}

export interface ListTemplatesResponse {
  templates: EmailTemplate[];
  total: number;
}

export interface PreviewTemplateResponse {
  renderedSubject: string;
  renderedBody: string;
}

export interface ViewSharedContentResponse {
  resourceType: string;
  resourceName: string;
  password?: string;
  fileContent?: string;
  fileName?: string;
  mimeType?: string;
}

// ==================== Share Service ====================

export const ShareService = {
  create: (data: CreateShareRequest, options?: RequestOptions) =>
    sharingApi.post<CreateShareResponse>('/shares', data, options),

  get: (id: string, options?: RequestOptions) =>
    sharingApi.get<{ share: SharedLink }>(`/shares/${id}`, options),

  list: (
    params?: {
      page?: number;
      pageSize?: number;
      resourceType?: string;
      recipientEmail?: string;
    },
    options?: RequestOptions,
  ) => {
    const query = new URLSearchParams();
    if (params?.page) query.set('page', String(params.page));
    if (params?.pageSize) query.set('pageSize', String(params.pageSize));
    if (params?.resourceType)
      query.set('resourceType', params.resourceType);
    if (params?.recipientEmail)
      query.set('recipientEmail', params.recipientEmail);
    const qs = query.toString();
    return sharingApi.get<ListSharesResponse>(
      `/shares${qs ? `?${qs}` : ''}`,
      options,
    );
  },

  revoke: (id: string, options?: RequestOptions) =>
    sharingApi.delete<void>(`/shares/${id}`, options),

  createPolicy: (
    shareLinkId: string,
    data: CreateSharePolicyRequest,
    options?: RequestOptions,
  ) =>
    sharingApi.post<{ policy: SharePolicy }>(
      `/shares/${shareLinkId}/policies`,
      data,
      options,
    ),

  listPolicies: (shareLinkId: string, options?: RequestOptions) =>
    sharingApi.get<{ policies: SharePolicy[] }>(
      `/shares/${shareLinkId}/policies`,
      options,
    ),

  deletePolicy: (
    shareLinkId: string,
    id: string,
    options?: RequestOptions,
  ) =>
    sharingApi.delete<void>(
      `/shares/${shareLinkId}/policies/${id}`,
      options,
    ),
};

// ==================== Template Service ====================

export const TemplateService = {
  create: (data: CreateTemplateRequest, options?: RequestOptions) =>
    sharingApi.post<{ template: EmailTemplate }>('/templates', data, options),

  get: (id: string, options?: RequestOptions) =>
    sharingApi.get<{ template: EmailTemplate }>(`/templates/${id}`, options),

  list: (
    params?: { page?: number; pageSize?: number },
    options?: RequestOptions,
  ) => {
    const query = new URLSearchParams();
    if (params?.page) query.set('page', String(params.page));
    if (params?.pageSize) query.set('pageSize', String(params.pageSize));
    const qs = query.toString();
    return sharingApi.get<ListTemplatesResponse>(
      `/templates${qs ? `?${qs}` : ''}`,
      options,
    );
  },

  update: (
    id: string,
    data: UpdateTemplateRequest,
    options?: RequestOptions,
  ) =>
    sharingApi.put<{ template: EmailTemplate }>(
      `/templates/${id}`,
      data,
      options,
    ),

  delete: (id: string, options?: RequestOptions) =>
    sharingApi.delete<void>(`/templates/${id}`, options),

  preview: (
    data: { subject: string; htmlBody: string },
    options?: RequestOptions,
  ) =>
    sharingApi.post<PreviewTemplateResponse>(
      '/templates/preview',
      data,
      options,
    ),
};
