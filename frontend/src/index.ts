import type { TangraModule } from './sdk';

import routes from './routes';
import { useSharingShareStore } from './stores/sharing-share.state';
import { useSharingTemplateStore } from './stores/sharing-template.state';
import enUS from './locales/en-US.json';

const sharingModule: TangraModule = {
  id: 'sharing',
  version: '1.0.0',
  routes,
  stores: {
    'sharing-share': useSharingShareStore,
    'sharing-template': useSharingTemplateStore,
  },
  locales: {
    'en-US': enUS,
  },
};

export default sharingModule;
