<script lang="ts" setup>
import { ref, computed } from 'vue';

import { useVbenDrawer } from 'shell/vben/common-ui';

import {
  Form,
  FormItem,
  Input,
  Button,
  notification,
  Textarea,
  Select,
  Descriptions,
  DescriptionsItem,
  Tag,
  Table,
  Divider,
  Popconfirm,
  Space,
} from 'ant-design-vue';

import { $t } from 'shell/locales';
import { useSharingShareStore } from '../../stores/sharing-share.state';
import { useSharingTemplateStore } from '../../stores/sharing-template.state';
import type {
  SharedLink,
  SharePolicy,
  SharePolicyType,
  SharePolicyMethod,
  CreateSharePolicyInput,
} from '../../api/services';

const shareStore = useSharingShareStore();
const templateStore = useSharingTemplateStore();

const data = ref<{
  mode: 'create' | 'view';
  row?: SharedLink;
}>();
const loading = ref(false);
const templateOptions = ref<Array<{ value: string; label: string }>>([]);

// Policy state for view mode
const policies = ref<SharePolicy[]>([]);
const policiesLoading = ref(false);
const showPolicyForm = ref(false);
const policyFormState = ref<{
  type: SharePolicyType;
  method: SharePolicyMethod;
  value: string;
  reason: string;
}>({
  type: 'SHARE_POLICY_TYPE_WHITELIST',
  method: 'SHARE_POLICY_METHOD_IP',
  value: '',
  reason: '',
});

// Policy state for create mode (inline policies)
const createPolicies = ref<CreateSharePolicyInput[]>([]);
const showCreatePolicyForm = ref(false);
const createPolicyFormState = ref<{
  type: SharePolicyType;
  method: SharePolicyMethod;
  value: string;
  reason: string;
}>({
  type: 'SHARE_POLICY_TYPE_WHITELIST',
  method: 'SHARE_POLICY_METHOD_IP',
  value: '',
  reason: '',
});

const formState = ref<{
  resourceType: string;
  resourceId: string;
  recipientEmail: string;
  message: string;
  templateId?: string;
}>({
  resourceType: 'RESOURCE_TYPE_SECRET',
  resourceId: '',
  recipientEmail: '',
  message: '',
  templateId: undefined,
});

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

const policyTypeOptions = computed(() => [
  {
    value: 'SHARE_POLICY_TYPE_WHITELIST',
    label: $t('sharing.page.policy.whitelist'),
  },
  {
    value: 'SHARE_POLICY_TYPE_BLACKLIST',
    label: $t('sharing.page.policy.blacklist'),
  },
]);

const policyMethodOptions = computed(() => [
  { value: 'SHARE_POLICY_METHOD_IP', label: $t('sharing.page.policy.methodIp') },
  { value: 'SHARE_POLICY_METHOD_MAC', label: $t('sharing.page.policy.methodMac') },
  { value: 'SHARE_POLICY_METHOD_REGION', label: $t('sharing.page.policy.methodRegion') },
  { value: 'SHARE_POLICY_METHOD_TIME', label: $t('sharing.page.policy.methodTime') },
  { value: 'SHARE_POLICY_METHOD_DEVICE', label: $t('sharing.page.policy.methodDevice') },
  { value: 'SHARE_POLICY_METHOD_NETWORK', label: $t('sharing.page.policy.methodNetwork') },
]);

const methodPlaceholders: Record<string, string> = {
  SHARE_POLICY_METHOD_IP: 'sharing.page.policy.valuePlaceholderIp',
  SHARE_POLICY_METHOD_NETWORK: 'sharing.page.policy.valuePlaceholderNetwork',
  SHARE_POLICY_METHOD_MAC: 'sharing.page.policy.valuePlaceholderMac',
  SHARE_POLICY_METHOD_REGION: 'sharing.page.policy.valuePlaceholderRegion',
  SHARE_POLICY_METHOD_TIME: 'sharing.page.policy.valuePlaceholderTime',
  SHARE_POLICY_METHOD_DEVICE: 'sharing.page.policy.valuePlaceholderDevice',
};

function getValuePlaceholder(method: string): string {
  const key = methodPlaceholders[method];
  return key ? $t(key) : '';
}

const policyColumns = computed(() => [
  {
    title: $t('sharing.page.policy.type'),
    dataIndex: 'type',
    key: 'type',
  },
  {
    title: $t('sharing.page.policy.method'),
    dataIndex: 'method',
    key: 'method',
  },
  {
    title: $t('sharing.page.policy.value'),
    dataIndex: 'value',
    key: 'value',
  },
  {
    title: $t('sharing.page.policy.reason'),
    dataIndex: 'reason',
    key: 'reason',
  },
  {
    title: '',
    key: 'actions',
    width: 80,
  },
]);

const title = computed(() => {
  return data.value?.mode === 'create'
    ? $t('sharing.page.link.create')
    : $t('sharing.page.link.view');
});

const isCreateMode = computed(() => data.value?.mode === 'create');
const isViewMode = computed(() => data.value?.mode === 'view');

function policyTypeLabel(type: string) {
  return type === 'SHARE_POLICY_TYPE_WHITELIST'
    ? $t('sharing.page.policy.whitelist')
    : $t('sharing.page.policy.blacklist');
}

function policyMethodLabel(method: string) {
  const opt = policyMethodOptions.value.find((o) => o.value === method);
  return opt?.label ?? method;
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

function resourceTypeToName(type: string | undefined) {
  const option = resourceTypeOptions.value.find((o) => o.value === type);
  return option?.label ?? type ?? '';
}

async function loadTemplates() {
  try {
    const resp = await templateStore.listTemplates({ page: 1, pageSize: 100 });
    templateOptions.value = (resp.templates ?? []).map((t) => ({
      value: t.id,
      label: `${t.name}${t.isDefault ? ' (Default)' : ''}`,
    }));
  } catch (e) {
    console.error('Failed to load templates:', e);
  }
}

async function loadPolicies(shareLinkId: string) {
  policiesLoading.value = true;
  try {
    const resp = await shareStore.listPolicies(shareLinkId);
    policies.value = resp.policies ?? [];
  } catch (e) {
    console.error('Failed to load policies:', e);
    policies.value = [];
  } finally {
    policiesLoading.value = false;
  }
}

async function handleAddPolicy() {
  if (!share.value) return;
  loading.value = true;
  try {
    await shareStore.createPolicy(share.value.id, {
      type: policyFormState.value.type,
      method: policyFormState.value.method,
      value: policyFormState.value.value,
      reason: policyFormState.value.reason || undefined,
    });
    notification.success({
      message: $t('sharing.page.policy.createSuccess'),
    });
    showPolicyForm.value = false;
    resetPolicyForm();
    await loadPolicies(share.value.id);
  } catch (e) {
    console.error('Failed to add policy:', e);
    notification.error({
      message: $t('ui.notification.create_failed'),
    });
  } finally {
    loading.value = false;
  }
}

async function handleDeletePolicy(policyId: string) {
  if (!share.value) return;
  try {
    await shareStore.deletePolicy(share.value.id, policyId);
    notification.success({
      message: $t('sharing.page.policy.deleteSuccess'),
    });
    await loadPolicies(share.value.id);
  } catch (e) {
    console.error('Failed to delete policy:', e);
  }
}

function resetPolicyForm() {
  policyFormState.value = {
    type: 'SHARE_POLICY_TYPE_WHITELIST',
    method: 'SHARE_POLICY_METHOD_IP',
    value: '',
    reason: '',
  };
}

// Create mode: add inline policy
function handleAddCreatePolicy() {
  createPolicies.value.push({
    type: createPolicyFormState.value.type,
    method: createPolicyFormState.value.method,
    value: createPolicyFormState.value.value,
    reason: createPolicyFormState.value.reason || undefined,
  });
  showCreatePolicyForm.value = false;
  createPolicyFormState.value = {
    type: 'SHARE_POLICY_TYPE_WHITELIST',
    method: 'SHARE_POLICY_METHOD_IP',
    value: '',
    reason: '',
  };
}

function handleRemoveCreatePolicy(index: number) {
  createPolicies.value.splice(index, 1);
}

async function handleSubmit() {
  loading.value = true;
  try {
    await shareStore.createShare({
      resourceType: formState.value.resourceType as
        | 'RESOURCE_TYPE_SECRET'
        | 'RESOURCE_TYPE_DOCUMENT',
      resourceId: formState.value.resourceId,
      recipientEmail: formState.value.recipientEmail,
      message: formState.value.message || undefined,
      templateId: formState.value.templateId,
      policies:
        createPolicies.value.length > 0 ? createPolicies.value : undefined,
    });
    notification.success({
      message: $t('sharing.page.link.createSuccess'),
    });
    drawerApi.close();
  } catch (e) {
    console.error('Failed to create share:', e);
    notification.error({
      message: $t('ui.notification.create_failed'),
    });
  } finally {
    loading.value = false;
  }
}

function resetForm() {
  formState.value = {
    resourceType: 'RESOURCE_TYPE_SECRET',
    resourceId: '',
    recipientEmail: '',
    message: '',
    templateId: undefined,
  };
  createPolicies.value = [];
  showCreatePolicyForm.value = false;
}

const [Drawer, drawerApi] = useVbenDrawer({
  onCancel() {
    drawerApi.close();
  },

  async onOpenChange(isOpen) {
    if (isOpen) {
      data.value = drawerApi.getData() as {
        mode: 'create' | 'view';
        row?: SharedLink;
      };

      if (data.value?.mode === 'create') {
        resetForm();
        await loadTemplates();
      } else if (data.value?.mode === 'view' && data.value.row) {
        showPolicyForm.value = false;
        resetPolicyForm();
        await loadPolicies(data.value.row.id);
      }
    }
  },
});

const share = computed(() => data.value?.row);
</script>

<template>
  <Drawer :title="title" :footer="false">
    <!-- View Mode -->
    <template v-if="share && isViewMode">
      <Descriptions :column="1" bordered size="small">
        <DescriptionsItem :label="$t('sharing.page.link.resourceType')">
          {{ resourceTypeToName(share.resourceType) }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('sharing.page.link.resourceName')">
          {{ share.resourceName || '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('sharing.page.link.recipientEmail')">
          {{ share.recipientEmail || '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('sharing.page.link.message')">
          {{ share.message || '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('sharing.page.link.status')">
          <Tag :color="statusToColor(share)">
            {{ statusToName(share) }}
          </Tag>
        </DescriptionsItem>
        <DescriptionsItem
          v-if="share.viewed && share.viewedAt"
          :label="$t('sharing.page.link.viewedAt')"
        >
          {{ share.viewedAt }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('sharing.page.link.createdAt')">
          {{ share.createTime || '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('sharing.page.link.token')">
          <span class="font-mono text-xs">{{ share.token }}</span>
        </DescriptionsItem>
      </Descriptions>

      <!-- Access Restrictions Section -->
      <Divider />
      <div class="mb-3 flex items-center justify-between">
        <h4 class="m-0 text-base font-medium">
          {{ $t('sharing.page.policy.title') }}
        </h4>
        <Button
          v-if="!share.revoked"
          size="small"
          type="primary"
          @click="showPolicyForm = !showPolicyForm"
        >
          {{ $t('sharing.page.policy.add') }}
        </Button>
      </div>

      <!-- Add Policy Form -->
      <div v-if="showPolicyForm" class="mb-4 rounded border border-gray-200 p-3">
        <Form layout="vertical" :model="policyFormState" @finish="handleAddPolicy">
          <div class="grid grid-cols-2 gap-3">
            <FormItem
              :label="$t('sharing.page.policy.type')"
              name="type"
              :rules="[{ required: true }]"
            >
              <Select
                v-model:value="policyFormState.type"
                :options="policyTypeOptions"
              />
            </FormItem>
            <FormItem
              :label="$t('sharing.page.policy.method')"
              name="method"
              :rules="[{ required: true }]"
            >
              <Select
                v-model:value="policyFormState.method"
                :options="policyMethodOptions"
              />
            </FormItem>
          </div>
          <FormItem
            :label="$t('sharing.page.policy.value')"
            name="value"
            :rules="[{ required: true, message: $t('ui.formRules.required') }]"
          >
            <Input
              v-model:value="policyFormState.value"
              :placeholder="getValuePlaceholder(policyFormState.method)"
            />
          </FormItem>
          <FormItem :label="$t('sharing.page.policy.reason')" name="reason">
            <Input v-model:value="policyFormState.reason" />
          </FormItem>
          <Space>
            <Button type="primary" html-type="submit" :loading="loading" size="small">
              {{ $t('sharing.page.policy.add') }}
            </Button>
            <Button size="small" @click="showPolicyForm = false">
              {{ $t('ui.actionTitle.cancel') }}
            </Button>
          </Space>
        </Form>
      </div>

      <!-- Policies Table -->
      <Table
        :columns="policyColumns"
        :data-source="policies"
        :loading="policiesLoading"
        :pagination="false"
        size="small"
        row-key="id"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'type'">
            <Tag :color="record.type === 'SHARE_POLICY_TYPE_WHITELIST' ? 'green' : 'red'">
              {{ policyTypeLabel(record.type) }}
            </Tag>
          </template>
          <template v-else-if="column.key === 'method'">
            {{ policyMethodLabel(record.method) }}
          </template>
          <template v-else-if="column.key === 'actions'">
            <Popconfirm
              :title="$t('sharing.page.policy.confirmDelete')"
              @confirm="handleDeletePolicy(record.id)"
            >
              <Button type="link" danger size="small">
                {{ $t('sharing.page.policy.delete') }}
              </Button>
            </Popconfirm>
          </template>
        </template>
      </Table>
    </template>

    <!-- Create Mode -->
    <template v-else-if="isCreateMode">
      <Form layout="vertical" :model="formState" @finish="handleSubmit">
        <FormItem
          :label="$t('sharing.page.link.resourceType')"
          name="resourceType"
          :rules="[{ required: true, message: $t('ui.formRules.required') }]"
        >
          <Select
            v-model:value="formState.resourceType"
            :options="resourceTypeOptions"
            :placeholder="$t('sharing.page.link.selectResourceType')"
          />
        </FormItem>

        <FormItem
          :label="$t('sharing.page.link.resourceId')"
          name="resourceId"
          :rules="[{ required: true, message: $t('ui.formRules.required') }]"
        >
          <Input
            v-model:value="formState.resourceId"
            :placeholder="$t('ui.placeholder.input')"
          />
        </FormItem>

        <FormItem
          :label="$t('sharing.page.link.recipientEmail')"
          name="recipientEmail"
          :rules="[
            { required: true, message: $t('ui.formRules.required') },
            { type: 'email', message: $t('ui.formRules.email') },
          ]"
        >
          <Input
            v-model:value="formState.recipientEmail"
            :placeholder="$t('sharing.page.link.recipientEmailPlaceholder')"
          />
        </FormItem>

        <FormItem :label="$t('sharing.page.link.message')" name="message">
          <Textarea
            v-model:value="formState.message"
            :rows="3"
            :maxlength="1024"
            :placeholder="$t('sharing.page.link.messagePlaceholder')"
          />
        </FormItem>

        <FormItem
          v-if="templateOptions.length > 0"
          :label="$t('sharing.menu.templates')"
          name="templateId"
        >
          <Select
            v-model:value="formState.templateId"
            :options="templateOptions"
            :placeholder="$t('sharing.page.link.selectTemplate')"
            allow-clear
          />
        </FormItem>

        <!-- Access Restrictions (Create Mode) -->
        <Divider />
        <div class="mb-3 flex items-center justify-between">
          <h4 class="m-0 text-base font-medium">
            {{ $t('sharing.page.policy.title') }}
          </h4>
          <Button
            size="small"
            type="dashed"
            @click="showCreatePolicyForm = !showCreatePolicyForm"
          >
            {{ $t('sharing.page.policy.add') }}
          </Button>
        </div>

        <!-- Inline Policy Form for Create Mode -->
        <div
          v-if="showCreatePolicyForm"
          class="mb-4 rounded border border-gray-200 p-3"
        >
          <div class="grid grid-cols-2 gap-3">
            <FormItem :label="$t('sharing.page.policy.type')">
              <Select
                v-model:value="createPolicyFormState.type"
                :options="policyTypeOptions"
              />
            </FormItem>
            <FormItem :label="$t('sharing.page.policy.method')">
              <Select
                v-model:value="createPolicyFormState.method"
                :options="policyMethodOptions"
              />
            </FormItem>
          </div>
          <FormItem :label="$t('sharing.page.policy.value')">
            <Input
              v-model:value="createPolicyFormState.value"
              :placeholder="getValuePlaceholder(createPolicyFormState.method)"
            />
          </FormItem>
          <FormItem :label="$t('sharing.page.policy.reason')">
            <Input v-model:value="createPolicyFormState.reason" />
          </FormItem>
          <Space>
            <Button
              size="small"
              type="primary"
              :disabled="!createPolicyFormState.value"
              @click="handleAddCreatePolicy"
            >
              {{ $t('sharing.page.policy.add') }}
            </Button>
            <Button size="small" @click="showCreatePolicyForm = false">
              {{ $t('ui.actionTitle.cancel') }}
            </Button>
          </Space>
        </div>

        <!-- Pending Policies List -->
        <div v-if="createPolicies.length > 0" class="mb-4">
          <Table
            :columns="policyColumns"
            :data-source="
              createPolicies.map((p, i) => ({ ...p, id: String(i) }))
            "
            :pagination="false"
            size="small"
            row-key="id"
          >
            <template #bodyCell="{ column, record, index }">
              <template v-if="column.key === 'type'">
                <Tag
                  :color="
                    record.type === 'SHARE_POLICY_TYPE_WHITELIST'
                      ? 'green'
                      : 'red'
                  "
                >
                  {{ policyTypeLabel(record.type) }}
                </Tag>
              </template>
              <template v-else-if="column.key === 'method'">
                {{ policyMethodLabel(record.method) }}
              </template>
              <template v-else-if="column.key === 'actions'">
                <Button
                  type="link"
                  danger
                  size="small"
                  @click="handleRemoveCreatePolicy(index)"
                >
                  {{ $t('sharing.page.policy.delete') }}
                </Button>
              </template>
            </template>
          </Table>
        </div>

        <FormItem>
          <Button type="primary" html-type="submit" :loading="loading" block>
            {{ $t('sharing.page.link.create') }}
          </Button>
        </FormItem>
      </Form>
    </template>
  </Drawer>
</template>
