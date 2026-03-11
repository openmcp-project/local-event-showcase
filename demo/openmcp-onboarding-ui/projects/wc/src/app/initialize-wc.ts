import { CrossplaneOnboardingComponent } from './crossplane-onboarding/crossplane-onboarding.component';
import { KROOnboardingComponent } from './kro-onboarding/kro-onboarding.component';
import { FluxOnboardingComponent } from './flux-onboarding/flux-onboarding.component';
import { OCMOnboardingComponent } from './ocm-onboarding/ocm-onboarding.component';
import { Injector, inject } from '@angular/core';
import { createCustomElement } from '@angular/elements';

export function initializeWC() {
  const source = import.meta.url;
  const injector = inject(Injector);

  const crossplaneEl = createCustomElement(CrossplaneOnboardingComponent, { injector });
  const kroEl = createCustomElement(KROOnboardingComponent, { injector });
  const fluxEl = createCustomElement(FluxOnboardingComponent, { injector });
  const ocmEl = createCustomElement(OCMOnboardingComponent, { injector });

  // @ts-expect-error global
  window.Luigi._registerWebcomponent(source, crossplaneEl);

  customElements.define('kro-onboarding', kroEl);
  customElements.define('flux-onboarding', fluxEl);
  customElements.define('ocm-onboarding', ocmEl);
}
