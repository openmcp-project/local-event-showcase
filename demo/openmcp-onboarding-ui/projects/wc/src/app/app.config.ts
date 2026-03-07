import { initializeWC } from './initialize-wc';
import { provideHttpClient } from '@angular/common/http';
import {
  ApplicationConfig,
  provideAppInitializer,
  provideZonelessChangeDetection,
} from '@angular/core';
import { provideNamedApollo } from 'apollo-angular';

export const appConfig: ApplicationConfig = {
  providers: [
    provideAppInitializer(() => {
      initializeWC();
    }),
    provideZonelessChangeDetection(),
    provideNamedApollo(() => ({})),
    provideHttpClient(),
  ],
};
