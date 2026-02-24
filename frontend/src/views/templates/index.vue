<script lang="ts" setup>
import type { VxeGridProps } from 'shell/adapter/vxe-table';

import { h } from 'vue';

import { Page, useVbenDrawer, type VbenFormProps } from 'shell/vben/common-ui';
import { LucideEye, LucideTrash, LucidePencil } from 'shell/vben/icons';

import { notification, Space, Button, Tag } from 'ant-design-vue';

import { useVbenVxeGrid } from 'shell/adapter/vxe-table';
import { $t } from 'shell/locales';
import { useSharingTemplateStore } from '../../stores/sharing-template.state';
import type { EmailTemplate } from '../../api/services';

import TemplateDrawer from './template-drawer.vue';

const templateStore = useSharingTemplateStore();

const formOptions: VbenFormProps = {
  collapsed: false,
  showCollapseButton: false,
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'query',
      label: $t('ui.table.search'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
  ],
};

const gridOptions: VxeGridProps<EmailTemplate> = {
  height: 'auto',
  stripe: false,
  toolbarConfig: {
    custom: true,
    export: true,
    import: false,
    refresh: true,
    zoom: true,
  },
  exportConfig: {},
  rowConfig: {
    isHover: true,
  },
  pagerConfig: {
    enabled: true,
    pageSize: 20,
    pageSizes: [10, 20, 50, 100],
  },

  proxyConfig: {
    ajax: {
      query: async ({ page }) => {
        const resp = await templateStore.listTemplates({
          page: page.currentPage,
          pageSize: page.pageSize,
        });
        return {
          items: resp.templates ?? [],
          total: resp.total ?? 0,
        };
      },
    },
  },

  columns: [
    { title: $t('ui.table.seq'), type: 'seq', width: 50 },
    {
      title: $t('sharing.page.template.name'),
      field: 'name',
      minWidth: 150,
    },
    {
      title: $t('sharing.page.template.subject'),
      field: 'subject',
      minWidth: 200,
    },
    {
      title: $t('sharing.page.template.isDefault'),
      field: 'isDefault',
      width: 100,
      slots: { default: 'isDefault' },
    },
    {
      title: $t('sharing.page.link.createdAt'),
      field: 'createTime',
      width: 160,
      sortable: true,
    },
    {
      title: $t('ui.table.action'),
      field: 'action',
      fixed: 'right',
      slots: { default: 'action' },
      width: 150,
    },
  ],
};

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions, formOptions });

const [TemplateDrawerComponent, templateDrawerApi] = useVbenDrawer({
  connectedComponent: TemplateDrawer,
  onOpenChange(isOpen: boolean) {
    if (!isOpen) {
      gridApi.query();
    }
  },
});

function openDrawer(
  row: EmailTemplate,
  mode: 'create' | 'edit' | 'view',
) {
  templateDrawerApi.setData({ row, mode });
  templateDrawerApi.open();
}

function handleView(row: EmailTemplate) {
  openDrawer(row, 'view');
}

function handleEdit(row: EmailTemplate) {
  openDrawer(row, 'edit');
}

function handleCreate() {
  openDrawer({} as EmailTemplate, 'create');
}

async function handleDelete(row: EmailTemplate) {
  if (!row.id) return;
  try {
    await templateStore.deleteTemplate(row.id);
    notification.success({
      message: $t('sharing.page.template.deleteSuccess'),
    });
    await gridApi.query();
  } catch {
    notification.error({ message: $t('ui.notification.delete_failed') });
  }
}
</script>

<template>
  <Page auto-content-height>
    <Grid :table-title="$t('sharing.page.template.title')">
      <template #toolbar-tools>
        <Button class="mr-2" type="primary" @click="handleCreate">
          {{ $t('sharing.page.template.create') }}
        </Button>
      </template>
      <template #isDefault="{ row }">
        <Tag :color="row.isDefault ? '#52C41A' : '#8C8C8C'">
          {{ row.isDefault ? 'Yes' : 'No' }}
        </Tag>
      </template>
      <template #action="{ row }">
        <Space>
          <Button
            type="link"
            size="small"
            :icon="h(LucideEye)"
            :title="$t('ui.button.view')"
            @click.stop="handleView(row)"
          />
          <Button
            type="link"
            size="small"
            :icon="h(LucidePencil)"
            :title="$t('ui.button.edit')"
            @click.stop="handleEdit(row)"
          />
          <a-popconfirm
            :cancel-text="$t('ui.button.cancel')"
            :ok-text="$t('ui.button.ok')"
            :title="$t('sharing.page.template.confirmDelete')"
            @confirm="handleDelete(row)"
          >
            <Button
              danger
              type="link"
              size="small"
              :icon="h(LucideTrash)"
              :title="$t('ui.button.delete', { moduleName: '' })"
            />
          </a-popconfirm>
        </Space>
      </template>
    </Grid>

    <TemplateDrawerComponent />
  </Page>
</template>
