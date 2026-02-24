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
  Switch,
  Descriptions,
  DescriptionsItem,
  Tag,
  Alert,
  Divider,
} from 'ant-design-vue';

import { $t } from 'shell/locales';
import { useSharingTemplateStore } from '../../stores/sharing-template.state';
import type { EmailTemplate } from '../../api/services';

const templateStore = useSharingTemplateStore();

const data = ref<{
  mode: 'create' | 'edit' | 'view';
  row?: EmailTemplate;
}>();
const loading = ref(false);
const previewLoading = ref(false);
const previewSubject = ref('');
const previewBody = ref('');
const showPreview = ref(false);

const formState = ref<{
  name: string;
  subject: string;
  htmlBody: string;
  isDefault: boolean;
}>({
  name: '',
  subject: '',
  htmlBody: '',
  isDefault: false,
});

const title = computed(() => {
  switch (data.value?.mode) {
    case 'create':
      return $t('sharing.page.template.create');
    case 'edit':
      return $t('sharing.page.template.edit');
    default:
      return $t('sharing.page.template.view');
  }
});

const isCreateMode = computed(() => data.value?.mode === 'create');
const isEditMode = computed(() => data.value?.mode === 'edit');
const isViewMode = computed(() => data.value?.mode === 'view');

async function handleSubmit() {
  loading.value = true;
  try {
    if (isCreateMode.value) {
      await templateStore.createTemplate({
        name: formState.value.name,
        subject: formState.value.subject,
        htmlBody: formState.value.htmlBody,
        isDefault: formState.value.isDefault,
      });
      notification.success({
        message: $t('sharing.page.template.createSuccess'),
      });
    } else if (isEditMode.value && data.value?.row?.id) {
      await templateStore.updateTemplate(data.value.row.id, {
        name: formState.value.name,
        subject: formState.value.subject,
        htmlBody: formState.value.htmlBody,
        isDefault: formState.value.isDefault,
      });
      notification.success({
        message: $t('sharing.page.template.updateSuccess'),
      });
    }
    drawerApi.close();
  } catch (e) {
    console.error('Failed to save template:', e);
    notification.error({
      message: isCreateMode.value
        ? $t('ui.notification.create_failed')
        : $t('ui.notification.update_failed'),
    });
  } finally {
    loading.value = false;
  }
}

async function handlePreview() {
  previewLoading.value = true;
  try {
    const resp = await templateStore.previewTemplate({
      subject: formState.value.subject,
      htmlBody: formState.value.htmlBody,
    });
    previewSubject.value = resp.renderedSubject;
    previewBody.value = resp.renderedBody;
    showPreview.value = true;
  } catch (e) {
    console.error('Failed to preview template:', e);
    notification.error({ message: 'Failed to preview template' });
  } finally {
    previewLoading.value = false;
  }
}

function resetForm() {
  formState.value = {
    name: '',
    subject: '',
    htmlBody: '',
    isDefault: false,
  };
  showPreview.value = false;
}

const [Drawer, drawerApi] = useVbenDrawer({
  onCancel() {
    drawerApi.close();
  },

  async onOpenChange(isOpen) {
    if (isOpen) {
      data.value = drawerApi.getData() as {
        mode: 'create' | 'edit' | 'view';
        row?: EmailTemplate;
      };

      if (data.value?.mode === 'create') {
        resetForm();
      } else if (data.value?.row) {
        formState.value = {
          name: data.value.row.name ?? '',
          subject: data.value.row.subject ?? '',
          htmlBody: data.value.row.htmlBody ?? '',
          isDefault: data.value.row.isDefault ?? false,
        };
        showPreview.value = false;
      }
    }
  },
});

const template = computed(() => data.value?.row);
</script>

<template>
  <Drawer :title="title" :footer="false">
    <!-- View Mode -->
    <template v-if="template && isViewMode">
      <Descriptions :column="1" bordered size="small">
        <DescriptionsItem :label="$t('sharing.page.template.name')">
          {{ template.name || '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('sharing.page.template.subject')">
          {{ template.subject || '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('sharing.page.template.isDefault')">
          <Tag :color="template.isDefault ? '#52C41A' : '#8C8C8C'">
            {{ template.isDefault ? 'Yes' : 'No' }}
          </Tag>
        </DescriptionsItem>
        <DescriptionsItem :label="$t('sharing.page.link.createdAt')">
          {{ template.createTime || '-' }}
        </DescriptionsItem>
      </Descriptions>

      <Divider />

      <div>
        <h4>{{ $t('sharing.page.template.htmlBody') }}</h4>
        <div
          class="border rounded p-4 mt-2"
          style="max-height: 400px; overflow: auto"
          v-html="template.htmlBody"
        />
      </div>
    </template>

    <!-- Create/Edit Mode -->
    <template v-else-if="isCreateMode || isEditMode">
      <Alert
        class="mb-4"
        type="info"
        :message="$t('sharing.page.template.variables')"
        :description="$t('sharing.page.template.variableHelp')"
        show-icon
      />

      <Form layout="vertical" :model="formState" @finish="handleSubmit">
        <FormItem
          :label="$t('sharing.page.template.name')"
          name="name"
          :rules="[{ required: true, message: $t('ui.formRules.required') }]"
        >
          <Input
            v-model:value="formState.name"
            :placeholder="$t('sharing.page.template.namePlaceholder')"
            :maxlength="255"
          />
        </FormItem>

        <FormItem
          :label="$t('sharing.page.template.subject')"
          name="subject"
          :rules="[{ required: true, message: $t('ui.formRules.required') }]"
        >
          <Input
            v-model:value="formState.subject"
            :placeholder="$t('sharing.page.template.subjectPlaceholder')"
            :maxlength="500"
          />
        </FormItem>

        <FormItem
          :label="$t('sharing.page.template.htmlBody')"
          name="htmlBody"
          :rules="[{ required: true, message: $t('ui.formRules.required') }]"
        >
          <Textarea
            v-model:value="formState.htmlBody"
            :rows="12"
            :placeholder="$t('sharing.page.template.htmlBodyPlaceholder')"
          />
        </FormItem>

        <FormItem :label="$t('sharing.page.template.isDefault')" name="isDefault">
          <Switch v-model:checked="formState.isDefault" />
        </FormItem>

        <FormItem>
          <Space>
            <Button type="primary" html-type="submit" :loading="loading">
              {{
                isCreateMode
                  ? $t('ui.button.create', { moduleName: '' })
                  : $t('ui.button.save')
              }}
            </Button>
            <Button :loading="previewLoading" @click="handlePreview">
              {{ $t('sharing.page.template.preview') }}
            </Button>
          </Space>
        </FormItem>
      </Form>

      <!-- Preview Section -->
      <template v-if="showPreview">
        <Divider />
        <h4>{{ $t('sharing.page.template.preview') }}</h4>
        <div class="mb-2">
          <strong>{{ $t('sharing.page.template.subject') }}:</strong>
          {{ previewSubject }}
        </div>
        <div
          class="border rounded p-4"
          style="max-height: 400px; overflow: auto"
          v-html="previewBody"
        />
      </template>
    </template>
  </Drawer>
</template>
