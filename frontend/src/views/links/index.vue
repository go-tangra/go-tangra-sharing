<script lang="ts" setup>
import type { VxeGridProps } from 'shell/adapter/vxe-table';

import { h, computed } from 'vue';

import { Page, useVbenDrawer, type VbenFormProps } from 'shell/vben/common-ui';
import { LucideEye, LucideBan } from 'shell/vben/icons';

import { notification, Space, Button, Tag } from 'ant-design-vue';

import { useVbenVxeGrid } from 'shell/adapter/vxe-table';
import { $t } from 'shell/locales';
import { useSharingShareStore } from '../../stores/sharing-share.state';
import type { SharedLink } from '../../api/services';

import LinkDrawer from './link-drawer.vue';

const shareStore = useSharingShareStore();

const resourceTypeOptions = computed(() => [
  {
    value: 'RESOURCE_TYPE_SECRET',
    label: $t('sharing.page.link.typeSecret'),
  },
  {
    value: 'RESOURCE_TYPE_DOCUMENT',
    label: $t('sharing.page.link.typeDocument'),
  },
]);

function resourceTypeToName(type: string | undefined) {
  const option = resourceTypeOptions.value.find((o) => o.value === type);
  return option?.label ?? type ?? '';
}

function statusToColor(row: SharedLink) {
  if (row.revoked) return '#FF4D4F';
  if (row.viewed) return '#1890FF';
  return '#52C41A';
}

function statusToName(row: SharedLink) {
  if (row.revoked) return $t('sharing.page.link.statusRevoked');
  if (row.viewed) return $t('sharing.page.link.statusViewed');
  return $t('sharing.page.link.statusActive');
}

const formOptions: VbenFormProps = {
  collapsed: false,
  showCollapseButton: false,
  submitOnEnter: true,
  schema: [
    {
      component: 'Select',
      fieldName: 'resourceType',
      label: $t('sharing.page.link.resourceType'),
      componentProps: {
        options: resourceTypeOptions,
        placeholder: $t('ui.placeholder.select'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'recipientEmail',
      label: $t('sharing.page.link.recipientEmail'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
  ],
};

const gridOptions: VxeGridProps<SharedLink> = {
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
      query: async ({ page }, formValues) => {
        const resp = await shareStore.listShares(
          { page: page.currentPage, pageSize: page.pageSize },
          {
            resourceType: formValues?.resourceType,
            recipientEmail: formValues?.recipientEmail,
          },
        );
        return {
          items: resp.shares ?? [],
          total: resp.total ?? 0,
        };
      },
    },
  },

  columns: [
    { title: $t('ui.table.seq'), type: 'seq', width: 50 },
    {
      title: $t('sharing.page.link.resourceType'),
      field: 'resourceType',
      width: 120,
      slots: { default: 'resourceType' },
    },
    {
      title: $t('sharing.page.link.resourceName'),
      field: 'resourceName',
      minWidth: 150,
    },
    {
      title: $t('sharing.page.link.recipientEmail'),
      field: 'recipientEmail',
      minWidth: 180,
    },
    {
      title: $t('sharing.page.link.status'),
      field: 'status',
      width: 100,
      slots: { default: 'status' },
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
      width: 120,
    },
  ],
};

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions, formOptions });

const [LinkDrawerComponent, linkDrawerApi] = useVbenDrawer({
  connectedComponent: LinkDrawer,
  onOpenChange(isOpen: boolean) {
    if (!isOpen) {
      gridApi.query();
    }
  },
});

function handleView(row: SharedLink) {
  linkDrawerApi.setData({ row, mode: 'view' });
  linkDrawerApi.open();
}


async function handleRevoke(row: SharedLink) {
  if (!row.id) return;
  try {
    await shareStore.revokeShare(row.id);
    notification.success({ message: $t('sharing.page.link.revokeSuccess') });
    await gridApi.query();
  } catch {
    notification.error({ message: $t('ui.notification.delete_failed') });
  }
}
</script>

<template>
  <Page auto-content-height>
    <Grid :table-title="$t('sharing.page.link.title')">
      <template #resourceType="{ row }">
        <Tag
          :color="
            row.resourceType === 'RESOURCE_TYPE_SECRET' ? '#722ED1' : '#1890FF'
          "
        >
          {{ resourceTypeToName(row.resourceType) }}
        </Tag>
      </template>
      <template #status="{ row }">
        <Tag :color="statusToColor(row)">
          {{ statusToName(row) }}
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
          <a-popconfirm
            v-if="!row.revoked"
            :cancel-text="$t('ui.button.cancel')"
            :ok-text="$t('ui.button.ok')"
            :title="$t('sharing.page.link.confirmRevoke')"
            @confirm="handleRevoke(row)"
          >
            <Button
              danger
              type="link"
              size="small"
              :icon="h(LucideBan)"
              :title="$t('sharing.page.link.revoke')"
            />
          </a-popconfirm>
        </Space>
      </template>
    </Grid>

    <LinkDrawerComponent />
  </Page>
</template>
