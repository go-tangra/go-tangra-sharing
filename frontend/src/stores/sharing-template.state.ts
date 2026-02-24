import { defineStore } from 'pinia';

import {
  TemplateService,
  type CreateTemplateRequest,
  type UpdateTemplateRequest,
  type EmailTemplate,
  type ListTemplatesResponse,
  type PreviewTemplateResponse,
} from '../api/services';

export const useSharingTemplateStore = defineStore(
  'sharing-template',
  () => {
    async function listTemplates(
      paging?: { page?: number; pageSize?: number },
    ): Promise<ListTemplatesResponse> {
      return await TemplateService.list(paging);
    }

    async function getTemplate(
      id: string,
    ): Promise<{ template: EmailTemplate }> {
      return await TemplateService.get(id);
    }

    async function createTemplate(
      data: CreateTemplateRequest,
    ): Promise<{ template: EmailTemplate }> {
      return await TemplateService.create(data);
    }

    async function updateTemplate(
      id: string,
      data: UpdateTemplateRequest,
    ): Promise<{ template: EmailTemplate }> {
      return await TemplateService.update(id, data);
    }

    async function deleteTemplate(id: string): Promise<void> {
      return await TemplateService.delete(id);
    }

    async function previewTemplate(data: {
      subject: string;
      htmlBody: string;
    }): Promise<PreviewTemplateResponse> {
      return await TemplateService.preview(data);
    }

    function $reset() {}

    return {
      $reset,
      listTemplates,
      getTemplate,
      createTemplate,
      updateTemplate,
      deleteTemplate,
      previewTemplate,
    };
  },
);
