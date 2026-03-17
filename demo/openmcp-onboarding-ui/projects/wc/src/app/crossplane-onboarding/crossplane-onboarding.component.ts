import {
  CrossplaneOnboardingService,
  CrossplaneCatalog,
  CrossplaneStatus,
  APIBindingDetail,
  PermissionClaim,
  AcceptablePermissionClaim,
} from '../services/crossplane-onboarding.service';
import { LuigiContext } from '../services/apollo-factory';
import {
  ChangeDetectionStrategy,
  Component,
  Input,
  OnDestroy,
  ViewEncapsulation,
  inject,
  signal,
} from '@angular/core';
import { Subscription } from 'rxjs';
import { LuigiClient } from '@luigi-project/client/luigi-element';
import { sendCustomMessage } from '@luigi-project/client/luigi-client';
import { ButtonComponent } from '@fundamental-ngx/core/button';
import { BusyIndicatorComponent } from '@fundamental-ngx/core/busy-indicator';
import { MessageStripComponent } from '@fundamental-ngx/core/message-strip';
import { IconComponent } from '@fundamental-ngx/core/icon';
import { SelectComponent, OptionComponent } from '@fundamental-ngx/core/select';

type OnboardingState =
  | 'loading'
  | 'activate'
  | 'activating'
  | 'configure'
  | 'creating'
  | 'provisioning'
  | 'active';

@Component({
  selector: 'app-crossplane-onboarding',
  standalone: true,
  imports: [
    ButtonComponent,
    BusyIndicatorComponent,
    MessageStripComponent,
    IconComponent,
    SelectComponent,
    OptionComponent,
  ],
  encapsulation: ViewEncapsulation.ShadowDom,
  changeDetection: ChangeDetectionStrategy.OnPush,
  styles: `
    :host {
      display: block;
      padding: 1rem;
      font-family: var(--sapFontFamily, '72', '72full', Arial, Helvetica, sans-serif);
    }

    .onboarding-card {
      background: var(--sapTile_Background, #fff);
      border: 1px solid var(--sapTile_BorderColor, #d9d9d9);
      border-radius: 0.5rem;
      padding: 1.5rem;
      max-width: 600px;
    }

    .card-header {
      display: flex;
      align-items: center;
      gap: 0.75rem;
      margin-bottom: 1rem;
    }

    .card-header h2 {
      margin: 0;
      font-size: var(--sapFontHeader3Size, 1.25rem);
      color: var(--sapTextColor, #32363a);
    }

    .card-description {
      color: var(--sapContent_LabelColor, #6a6d70);
      font-size: var(--sapFontSize, 0.875rem);
      margin-bottom: 1.5rem;
      line-height: 1.5;
    }

    .config-section {
      background: var(--sapList_Background, #fff);
      border: 1px solid var(--sapList_BorderColor, #e4e4e4);
      border-radius: 0.25rem;
      padding: 1rem;
      margin-bottom: 1.5rem;
    }

    .config-row {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 0.5rem 0;
    }

    .config-row + .config-row {
      border-top: 1px solid var(--sapList_BorderColor, #e4e4e4);
    }

    .config-label {
      display: flex;
      align-items: center;
      gap: 0.5rem;
      color: var(--sapContent_LabelColor, #6a6d70);
      font-size: var(--sapFontSmallSize, 0.75rem);
    }

    .config-value {
      font-weight: bold;
      color: var(--sapTextColor, #32363a);
    }

    .config-field {
      display: flex;
      flex-direction: column;
      gap: 0.25rem;
      padding: 0.5rem 0;
    }

    .config-field + .config-field {
      border-top: 1px solid var(--sapList_BorderColor, #e4e4e4);
    }

    .config-field label {
      color: var(--sapContent_LabelColor, #6a6d70);
      font-size: var(--sapFontSmallSize, 0.75rem);
    }

    .loading-container {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 1rem;
      padding: 2rem;
    }

    .status-badge {
      display: inline-flex;
      align-items: center;
      gap: 0.5rem;
      padding: 0.25rem 0.75rem;
      border-radius: 1rem;
      font-size: var(--sapFontSmallSize, 0.75rem);
      font-weight: bold;
      background: var(--sapPositiveElementColor, #107e3e);
      color: #fff;
    }

    .status-badge.provisioning {
      background: var(--sapInformationColor, #0a6ed1);
    }

    .provisioning-card {
      background: var(--sapTile_Background, #fff);
      border: 1px solid var(--sapTile_BorderColor, #d9d9d9);
      border-radius: 0.5rem;
      padding: 1.5rem;
      max-width: 600px;
    }

    .provisioning-header {
      display: flex;
      align-items: center;
      gap: 0.75rem;
      margin-bottom: 1rem;
    }

    .provisioning-header h2 {
      margin: 0;
      font-size: var(--sapFontHeader3Size, 1.25rem);
      color: var(--sapTextColor, #32363a);
    }

    .provisioning-status {
      display: flex;
      align-items: center;
      gap: 1rem;
      padding: 1rem;
      background: var(--sapInformationBackground, #e5f0fa);
      border: 1px solid var(--sapInformationBorderColor, #0a6ed1);
      border-radius: 0.25rem;
      margin-bottom: 1rem;
    }

    .provisioning-status .phase-text {
      font-size: var(--sapFontSize, 0.875rem);
      color: var(--sapTextColor, #32363a);
    }

    .provisioning-status .phase-label {
      font-weight: bold;
      color: var(--sapInformationColor, #0a6ed1);
    }
  `,
  template: `
    @if (binding() && (pendingClaims().length > 0 || acceptedClaims().length > 0)) {
      <div class="onboarding-card" style="margin-bottom: 1rem;">
        <div class="card-header">
          <fd-icon glyph="key"></fd-icon>
          <h2>Permission Claims</h2>
        </div>
        <div class="card-description">
          The Crossplane service requests access to the following resources in your workspace.
        </div>
        <div class="config-section">
          @for (claim of acceptedClaims(); track claim.resource + claim.group) {
            <div class="config-row">
              <span class="config-label">
                <fd-icon glyph="accept" style="color: var(--sapPositiveColor)"></fd-icon>
                {{ claim.group || 'core' }} / {{ claim.resource }}
              </span>
              <span class="config-value" style="color: var(--sapPositiveTextColor)">Accepted</span>
            </div>
          }
          @for (claim of pendingClaims(); track claim.resource + claim.group) {
            <div class="config-row">
              <span class="config-label">
                <fd-icon glyph="pending"></fd-icon>
                {{ claim.group || 'core' }} / {{ claim.resource }}
              </span>
              <span class="config-value">
                <button fd-button fdType="transparent" label="Accept"
                  [disabled]="approvingClaim() === (claim.resource + claim.group)"
                  (click)="onAcceptClaim(claim)">
                </button>
              </span>
            </div>
          }
        </div>
      </div>
    }

    @switch (state()) {
      @case ('loading') {
        <div class="loading-container">
          <fd-busy-indicator [loading]="true" size="m"></fd-busy-indicator>
          <span>Checking Crossplane status...</span>
        </div>
      }

      @case ('activate') {
        <div class="onboarding-card">
          <div class="card-header">
            <fd-icon glyph="activate"></fd-icon>
            <h2>Enable Crossplane</h2>
          </div>
          <div class="card-description">
            Crossplane extends your workspace with cloud-native infrastructure management.
            Activate the Crossplane API to start provisioning and managing cloud resources
            directly from your workspace.
          </div>
          <button fd-button label="Start using Crossplane" fdType="emphasized"
            (click)="onActivate()"></button>
        </div>
      }

      @case ('activating') {
        <div class="loading-container">
          <fd-busy-indicator [loading]="true" size="m"></fd-busy-indicator>
          <span>Activating Crossplane API...</span>
        </div>
      }

      @case ('configure') {
        <div class="onboarding-card">
          <div class="card-header">
            <fd-icon glyph="settings"></fd-icon>
            <h2>Configure Crossplane</h2>
          </div>
          <div class="card-description">
            The Crossplane API is active. Configure your Crossplane installation with the
            following settings:
          </div>
          @if (catalog()) {
            <div class="config-section">
              <div class="config-field">
                <label>Crossplane Version</label>
                <fd-select [value]="selectedVersion()" (valueChange)="selectedVersion.set($event)"
                  placeholder="Select version">
                  @for (v of catalog()!.spec.versions; track v.version) {
                    <fd-option [value]="v.version">{{ v.version }}</fd-option>
                  }
                </fd-select>
              </div>
              @for (provider of catalog()!.spec.providers; track provider.name) {
                <div class="config-field">
                  <label>{{ provider.name }}</label>
                  <fd-select [value]="selectedProviderVersions()[provider.name]"
                    (valueChange)="onProviderVersionChange(provider.name, $event)"
                    placeholder="Select version">
                    @for (pv of provider.versions; track pv) {
                      <fd-option [value]="pv">{{ pv }}</fd-option>
                    }
                  </fd-select>
                </div>
              }
            </div>
          } @else {
            <div class="config-section">
              <div class="loading-container">
                <fd-busy-indicator [loading]="true" size="s"></fd-busy-indicator>
                <span>Loading available versions...</span>
              </div>
            </div>
          }
          <button fd-button label="Confirm and Install" fdType="emphasized"
            [disabled]="!catalog() || !selectedVersion()"
            (click)="onConfigure()"></button>
        </div>
      }

      @case ('creating') {
        <div class="loading-container">
          <fd-busy-indicator [loading]="true" size="m"></fd-busy-indicator>
          <span>Installing Crossplane...</span>
        </div>
      }

      @case ('provisioning') {
        <div class="provisioning-card">
          <div class="provisioning-header">
            <fd-busy-indicator [loading]="true" size="s"></fd-busy-indicator>
            <h2>Crossplane is provisioning</h2>
          </div>
          <div class="provisioning-status">
            <div>
              <div class="phase-text">Your Crossplane instance is being set up. This may take a few minutes.</div>
              @if (crossplane()?.status?.phase) {
                <div class="phase-label" style="margin-top: 0.5rem">
                  Phase: {{ crossplane()!.status!.phase }}
                </div>
              }
            </div>
          </div>
          @if (crossplane()) {
            <div class="config-section">
              <div class="config-row">
                <span class="config-label">Version</span>
                <span class="config-value">{{ crossplane()!.spec?.version }}</span>
              </div>
              @for (provider of crossplane()!.spec?.providers ?? []; track provider.name) {
                <div class="config-row">
                  <span class="config-label">Provider</span>
                  <span class="config-value">{{ provider.name }} {{ provider.version }}</span>
                </div>
              }
              <div class="config-row">
                <span class="config-label">Status</span>
                <span class="config-value">
                  <span class="status-badge provisioning">{{ crossplane()!.status?.phase ?? 'Pending' }}</span>
                </span>
              </div>
            </div>
          }
        </div>
      }

      @case ('active') {
        <div class="onboarding-card">
          <div class="card-header">
            <fd-icon glyph="sys-enter-2"></fd-icon>
            <h2>Crossplane Active</h2>
          </div>
          @if (crossplane()) {
            <div class="config-section">
              <div class="config-row">
                <span class="config-label">Version</span>
                <span class="config-value">{{ crossplane()!.spec?.version }}</span>
              </div>
              @for (provider of crossplane()!.spec?.providers ?? []; track provider.name) {
                <div class="config-row">
                  <span class="config-label">Provider</span>
                  <span class="config-value">{{ provider.name }} {{ provider.version }}</span>
                </div>
              }
              <div class="config-row">
                <span class="config-label">Status</span>
                <span class="config-value">
                  <span class="status-badge">{{ crossplane()!.status?.phase ?? 'Unknown' }}</span>
                </span>
              </div>
            </div>
          }
          <fd-message-strip type="success" [dismissible]="false">
            Crossplane is installed and running in your workspace.
          </fd-message-strip>
        </div>
      }
    }

    @if (error()) {
      <fd-message-strip type="error" [dismissible]="true" (dismiss)="error.set('')"
        style="margin-top: 1rem; max-width: 600px;">
        {{ error() }}
      </fd-message-strip>
    }
  `,
})
export class CrossplaneOnboardingComponent implements OnDestroy {
  private onboardingService = inject(CrossplaneOnboardingService);
  private watchSub?: Subscription;
  private bindingWatchSub?: Subscription;
  private luigiContext!: LuigiContext;

  state = signal<OnboardingState>('loading');
  error = signal('');
  crossplane = signal<CrossplaneStatus | null>(null);
  catalog = signal<CrossplaneCatalog | null>(null);
  selectedVersion = signal('');
  selectedProviderVersions = signal<Record<string, string>>({});

  binding = signal<APIBindingDetail | null>(null);
  pendingClaims = signal<PermissionClaim[]>([]);
  acceptedClaims = signal<AcceptablePermissionClaim[]>([]);
  approvingClaim = signal('');

  @Input()
  LuigiClient!: LuigiClient;

  @Input()
  set context(ctx: LuigiContext) {
    this.luigiContext = ctx;
    this.onboardingService.initialize(ctx);
    this.checkState();
  }

  ngOnDestroy(): void {
    this.watchSub?.unsubscribe();
    this.bindingWatchSub?.unsubscribe();
  }

  onActivate(): void {
    this.state.set('activating');
    this.error.set('');
    this.onboardingService.createAPIBinding('crossplane.services.opencp.cloud').subscribe({
      next: () => this.pollAPIBindingReady(),
      error: (err) => {
        this.error.set(`Failed to create API binding: ${err.message}`);
        this.state.set('activate');
      },
    });
  }

  onProviderVersionChange(providerName: string, version: string): void {
    this.selectedProviderVersions.update((prev) => ({
      ...prev,
      [providerName]: version,
    }));
  }

  onConfigure(): void {
    this.state.set('creating');
    this.error.set('');
    const providers = Object.entries(this.selectedProviderVersions())
      .filter(([, version]) => !!version)
      .map(([name, version]) => ({ name, version }));
    this.onboardingService
      .createCrossplane(this.selectedVersion(), providers)
      .subscribe({
        next: () => this.startWatchingCrossplane(),
        error: (err) => {
          this.error.set(`Failed to create Crossplane: ${err.message}`);
          this.state.set('configure');
        },
      });
  }

  onAcceptClaim(claim: PermissionClaim): void {
    const currentBinding = this.binding();
    if (!currentBinding) return;

    this.approvingClaim.set(claim.resource + claim.group);
    this.error.set('');
    this.onboardingService
      .acceptPermissionClaim(currentBinding.metadata.name, currentBinding, claim)
      .subscribe({
        next: () => {
          this.approvingClaim.set('');
        },
        error: (err) => {
          this.error.set(`Failed to accept permission claim: ${err.message}`);
          this.approvingClaim.set('');
        },
      });
  }

  private checkState(): void {
    this.state.set('loading');
    this.onboardingService.checkAPIBinding().subscribe({
      next: (binding) => {
        if (!binding) {
          this.state.set('activate');
        } else {
          this.binding.set(binding);
          this.computeClaims(binding);
          this.startWatchingBinding();
          this.checkCrossplaneState();
        }
      },
      error: (err) => {
        this.error.set(`Failed to check API binding: ${err.message}`);
        this.state.set('activate');
      },
    });
  }

  private computeClaims(binding: APIBindingDetail): void {
    const exportClaims = binding.status?.exportPermissionClaims ?? [];
    const specClaims = binding.spec?.permissionClaims ?? [];

    const accepted = specClaims.filter((c) => c.state === 'Accepted');
    this.acceptedClaims.set(accepted);

    const pending = exportClaims.filter(
      (ec) =>
        !accepted.some(
          (ac) =>
            ac.group === ec.group &&
            ac.resource === ec.resource &&
            ac.identityHash === ec.identityHash,
        ),
    );
    this.pendingClaims.set(pending);
  }

  private startWatchingBinding(): void {
    this.bindingWatchSub?.unsubscribe();
    this.bindingWatchSub = this.onboardingService
      .watchAPIBinding('crossplane.services.opencp.cloud')
      .subscribe({
        next: (event) => {
          this.binding.set(event.object);
          this.computeClaims(event.object);
        },
        error: () => {
          // Subscription failed silently — query-based flow still works
        },
      });
  }

  private checkCrossplaneState(): void {
    this.onboardingService.checkCrossplane().subscribe({
      next: (cp) => {
        if (cp) {
          this.crossplane.set(cp);
          if (cp.status?.phase === 'Ready') {
            this.state.set('active');
          } else {
            this.startWatchingCrossplane();
          }
        } else {
          this.state.set('configure');
          this.loadCatalog();
        }
      },
      error: () => {
        this.state.set('configure');
        this.loadCatalog();
      },
    });
  }

  private loadCatalog(): void {
    // TODO: Replace with catalog query once CachedResource virtual storage is working
    const hardcodedCatalog: CrossplaneCatalog = {
      metadata: { name: 'default' },
      spec: {
        versions: [{ version: 'v1.20.1' }],
        providers: [
          { name: 'provider-kubernetes', versions: ['v0.15.0'] },
        ],
      },
    };
    this.catalog.set(hardcodedCatalog);
    this.selectedVersion.set(hardcodedCatalog.spec.versions[0].version);
    const providerDefaults: Record<string, string> = {};
    for (const provider of hardcodedCatalog.spec.providers) {
      if (provider.versions.length > 0) {
        providerDefaults[provider.name] = provider.versions[0];
      }
    }
    this.selectedProviderVersions.set(providerDefaults);
  }

  private startWatchingCrossplane(): void {
    this.state.set('provisioning');
    this.watchSub?.unsubscribe();
    this.watchSub = this.onboardingService.watchCrossplane().subscribe({
      next: (event) => {
        this.crossplane.set(event.object);
        if (event.object.status?.phase === 'Ready') {
          this.state.set('active');
          this.sendPortalReloadMessage();
          this.watchSub?.unsubscribe();
        } else {
          this.state.set('provisioning');
        }
      },
      error: (err) => {
        this.error.set(`Watch error: ${err.message}`);
      },
    });
  }

  private sendPortalReloadMessage(): void {
    const entityType = this.luigiContext?.entityType ?? '';
    sendCustomMessage({
      id: 'openmfp.reload-luigi-config',
      origin: 'CrossplaneOnboarding',
      action: 'provisionCrossplane',
      entity: entityType,
      context: {
        [entityType]: this.luigiContext?.entityName,
        user: this.luigiContext?.userId,
      },
    });
  }

  private pollAPIBindingReady(): void {
    this.onboardingService.checkAPIBinding().subscribe({
      next: (binding) => {
        if (binding?.status?.phase === 'Bound') {
          this.binding.set(binding);
          this.computeClaims(binding);
          this.startWatchingBinding();
          this.checkCrossplaneState();
        } else {
          setTimeout(() => this.pollAPIBindingReady(), 2000);
        }
      },
      error: () => {
        setTimeout(() => this.pollAPIBindingReady(), 2000);
      },
    });
  }
}
