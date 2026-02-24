import { defineStore } from 'pinia';

import {
  ShareService,
  type CreateShareRequest,
  type CreateShareResponse,
  type CreateSharePolicyRequest,
  type ListSharesResponse,
  type SharePolicy,
  type SharedLink,
} from '../api/services';

export const useSharingShareStore = defineStore('sharing-share', () => {
  async function listShares(
    paging?: { page?: number; pageSize?: number },
    filters?: {
      resourceType?: string;
      recipientEmail?: string;
    } | null,
  ): Promise<ListSharesResponse> {
    return await ShareService.list({
      page: paging?.page,
      pageSize: paging?.pageSize,
      resourceType: filters?.resourceType,
      recipientEmail: filters?.recipientEmail,
    });
  }

  async function getShare(id: string): Promise<{ share: SharedLink }> {
    return await ShareService.get(id);
  }

  async function createShare(
    data: CreateShareRequest,
  ): Promise<CreateShareResponse> {
    return await ShareService.create(data);
  }

  async function revokeShare(id: string): Promise<void> {
    return await ShareService.revoke(id);
  }

  async function createPolicy(
    shareLinkId: string,
    data: CreateSharePolicyRequest,
  ): Promise<{ policy: SharePolicy }> {
    return await ShareService.createPolicy(shareLinkId, data);
  }

  async function listPolicies(
    shareLinkId: string,
  ): Promise<{ policies: SharePolicy[] }> {
    return await ShareService.listPolicies(shareLinkId);
  }

  async function deletePolicy(
    shareLinkId: string,
    id: string,
  ): Promise<void> {
    return await ShareService.deletePolicy(shareLinkId, id);
  }

  function $reset() {}

  return {
    $reset,
    listShares,
    getShare,
    createShare,
    revokeShare,
    createPolicy,
    listPolicies,
    deletePolicy,
  };
});
