import { CrossplaneOnboardingComponent } from './crossplane-onboarding/crossplane-onboarding.component';
import { Injector, inject } from '@angular/core';
import { createCustomElement } from '@angular/elements';

export function initializeWC() {
  const source = import.meta.url;
  const injector = inject(Injector);
  const el = createCustomElement(CrossplaneOnboardingComponent, { injector });
  // @ts-expect-error global
  window.Luigi._registerWebcomponent(source, el);
}
