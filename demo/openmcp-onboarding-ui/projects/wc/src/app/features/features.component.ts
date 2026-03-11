import {
  ChangeDetectionStrategy,
  Component,
  Input,
  OnDestroy,
  ViewEncapsulation,
  inject,
  signal,
} from '@angular/core';
import { Observable, Subscription, forkJoin, map } from 'rxjs';
import { LuigiClient } from '@luigi-project/client/luigi-element';
import { sendCustomMessage } from '@luigi-project/client/luigi-client';
import { ButtonComponent } from '@fundamental-ngx/core/button';
import { BusyIndicatorComponent } from '@fundamental-ngx/core/busy-indicator';
import { MessageStripComponent } from '@fundamental-ngx/core/message-strip';
import { IconComponent } from '@fundamental-ngx/core/icon';
import { SelectComponent, OptionComponent } from '@fundamental-ngx/core/select';
import { LuigiContext } from '../services/apollo-factory';
import {
  CrossplaneOnboardingService,
  APIBindingDetail,
  PermissionClaim,
  AcceptablePermissionClaim,
  CrossplaneStatus,
  CrossplaneCatalog,
} from '../services/crossplane-onboarding.service';
import { KROOnboardingService, KROStatus } from '../services/kro-onboarding.service';
import { FluxOnboardingService, FluxStatus } from '../services/flux-onboarding.service';
import { OCMOnboardingService, OCMControllerStatus } from '../services/ocm-onboarding.service';

type FeaturesState = 'loading' | 'activate' | 'activating' | 'features';
type ToolState = 'not-enabled' | 'configuring' | 'creating' | 'provisioning' | 'active' | 'disabling';

interface ToolCard {
  id: string;
  name: string;
  icon: string;
  logoUrl?: string;
  textLogo?: string;
  description: string;
  state: ToolState;
}

interface VersionEntry {
  version: string;
  chartVersion?: string;
}

interface ToolWatchEvent {
  object: { status?: { phase?: string } };
}

@Component({
  selector: 'app-features',
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

    .config-row + .config-row,
    .config-field + .config-field {
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

    .config-field label {
      color: var(--sapContent_LabelColor, #6a6d70);
      font-size: var(--sapFontSmallSize, 0.75rem);
    }

    .features-header {
      margin-bottom: 1.5rem;
    }

    .features-header h2 {
      margin: 0 0 0.5rem 0;
      font-size: var(--sapFontHeader2Size, 1.5rem);
      color: var(--sapTextColor, #32363a);
    }

    .features-header p {
      margin: 0;
      color: var(--sapContent_LabelColor, #6a6d70);
      font-size: var(--sapFontSize, 0.875rem);
    }

    .tiles-grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
      gap: 1rem;
    }

    .tile {
      background: var(--sapTile_Background, #fff);
      border: 1px solid var(--sapTile_BorderColor, #d9d9d9);
      border-radius: 0.5rem;
      padding: 1.5rem;
      display: flex;
      flex-direction: column;
      gap: 0.75rem;
      transition: box-shadow 0.15s;
    }

    .tile:hover {
      box-shadow: 0 0 0 1px var(--sapSelectedColor, #0854a0);
    }

    .tile-header {
      display: flex;
      align-items: center;
      gap: 0.75rem;
    }

    .tile-header h3 {
      margin: 0;
      font-size: var(--sapFontHeader3Size, 1.25rem);
      color: var(--sapTextColor, #32363a);
    }

    .tool-logo {
      width: 28px;
      height: 28px;
      object-fit: contain;
    }

    .tool-text-logo {
      font-size: 1.25rem;
      font-weight: 800;
      color: var(--sapTextColor, #32363a);
      letter-spacing: -0.02em;
      line-height: 28px;
    }

    .tile-description {
      color: var(--sapContent_LabelColor, #6a6d70);
      font-size: var(--sapFontSmallSize, 0.75rem);
      line-height: 1.5;
      flex: 1;
    }

    .tile-footer {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-top: 0.5rem;
    }

    .status-badge {
      display: inline-flex;
      align-items: center;
      gap: 0.25rem;
      padding: 0.125rem 0.5rem;
      border-radius: 1rem;
      font-size: var(--sapFontSmallSize, 0.75rem);
      font-weight: bold;
    }

    .status-badge.active {
      background: var(--sapPositiveElementColor, #107e3e);
      color: #fff;
    }

    .status-badge.provisioning,
    .status-badge.creating {
      background: var(--sapInformationColor, #0a6ed1);
      color: #fff;
    }

    .status-badge.not-enabled {
      background: var(--sapNeutralElementColor, #6a6d70);
      color: #fff;
    }

    .status-badge.disabling {
      background: var(--sapCriticalElementColor, #e9730c);
      color: #fff;
    }

    .status-badge.configuring {
      background: var(--sapNeutralBackground, #eee);
      color: var(--sapTextColor, #32363a);
    }

    .loading-container {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 1rem;
      padding: 2rem;
    }

    .inline-loading {
      display: flex;
      align-items: center;
      gap: 0.75rem;
      padding: 0.5rem 0;
    }

    .provisioning-status {
      display: flex;
      align-items: center;
      gap: 1rem;
      padding: 1rem;
      background: var(--sapInformationBackground, #e5f0fa);
      border: 1px solid var(--sapInformationBorderColor, #0a6ed1);
      border-radius: 0.25rem;
    }

    .phase-label {
      font-weight: bold;
      color: var(--sapInformationColor, #0a6ed1);
    }

    .drawer-panel {
      position: fixed;
      top: 0;
      right: 0;
      bottom: 0;
      width: 420px;
      max-width: 90vw;
      background: var(--sapBackgroundColor, #f7f7f7);
      box-shadow: -2px 0 8px rgba(0, 0, 0, 0.15);
      z-index: 1000;
      transform: translateX(100%);
      transition: transform 0.25s ease;
      display: flex;
      flex-direction: column;
    }

    .drawer-panel.open {
      transform: translateX(0);
    }

    .drawer-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 1rem 1.5rem;
      border-bottom: 1px solid var(--sapList_BorderColor, #e4e4e4);
      background: var(--sapTile_Background, #fff);
    }

    .drawer-header-title {
      display: flex;
      align-items: center;
      gap: 0.75rem;
    }

    .drawer-header h2 {
      margin: 0;
      font-size: var(--sapFontHeader3Size, 1.25rem);
      color: var(--sapTextColor, #32363a);
    }

    .drawer-body {
      flex: 1;
      overflow-y: auto;
      padding: 1.5rem;
    }

    .drawer-footer {
      padding: 1rem 1.5rem;
      border-top: 1px solid var(--sapList_BorderColor, #e4e4e4);
      background: var(--sapTile_Background, #fff);
      display: flex;
      gap: 0.5rem;
    }
  `,
  template: `
    @switch (state()) {
      @case ('loading') {
        <div class="loading-container">
          <fd-busy-indicator [loading]="true" size="m"></fd-busy-indicator>
          <span>Checking OpenMCP services...</span>
        </div>
      }

      @case ('activate') {
        <div class="onboarding-card">
          <div class="card-header">
            <fd-icon glyph="activate"></fd-icon>
            <h2>Enable OpenMCP Services</h2>
          </div>
          <div class="card-description">
            Activate the OpenMCP service APIs to start using tools like Crossplane, Flux, KRO,
            and OCM Controller in your workspace.
          </div>
          <button fd-button label="Enable Services" fdType="emphasized"
            (click)="onActivate()"></button>
        </div>
      }

      @case ('activating') {
        <div class="loading-container">
          <fd-busy-indicator [loading]="true" size="m"></fd-busy-indicator>
          <span>Activating OpenMCP services...</span>
        </div>
      }

      @case ('features') {
        <div class="features-header">
          <h2>Features</h2>
          <p>Manage the tools and services available in your workspace.</p>
        </div>
        <div class="tiles-grid">
          @for (tool of tools(); track tool.id) {
            <div class="tile">
              <div class="tile-header">
                @if (tool.logoUrl) {
                  <img class="tool-logo" [src]="tool.logoUrl" [alt]="tool.name + ' logo'" />
                } @else if (tool.textLogo) {
                  <span class="tool-text-logo">{{ tool.textLogo }}</span>
                } @else {
                  <fd-icon [glyph]="tool.icon"></fd-icon>
                }
                <h3>{{ tool.name }}</h3>
              </div>
              <div class="tile-description">{{ tool.description }}</div>
              <div class="tile-footer">
                <span class="status-badge" [class]="tool.state">
                  @switch (tool.state) {
                    @case ('active') { Active }
                    @case ('provisioning') { Provisioning }
                    @case ('creating') { Installing }
                    @case ('configuring') { Configuring }
                    @case ('disabling') { Disabling }
                    @case ('not-enabled') { Not Enabled }
                  }
                </span>
                @if (tool.state === 'not-enabled') {
                  <button fd-button fdType="emphasized" label="Enable"
                    (click)="onEnableTool(tool.id)"></button>
                } @else if (tool.state === 'provisioning' || tool.state === 'creating' || tool.state === 'disabling') {
                  <fd-busy-indicator [loading]="true" size="s"></fd-busy-indicator>
                } @else if (tool.state === 'active') {
                  <button fd-button fdType="transparent" label="Disable"
                    (click)="onDisableTool(tool.id)"></button>
                }
              </div>
            </div>
          }
        </div>

        <!-- Drawer -->
        @if (drawerToolId()) {
          <div class="drawer-panel" [class.open]="drawerOpen()">
            <div class="drawer-header">
              <div class="drawer-header-title">
                @if (drawerTool()?.logoUrl) {
                  <img class="tool-logo" [src]="drawerTool()!.logoUrl!" [alt]="drawerTool()!.name + ' logo'" />
                } @else if (drawerTool()?.textLogo) {
                  <span class="tool-text-logo">{{ drawerTool()!.textLogo }}</span>
                } @else {
                  <fd-icon [glyph]="drawerTool()?.icon ?? 'settings'"></fd-icon>
                }
                <h2>Enable {{ drawerTool()?.name }}</h2>
              </div>
              <button fd-button fdType="transparent" glyph="decline" (click)="closeDrawer()"></button>
            </div>
            <div class="drawer-body">
              @if (drawerTool(); as tool) {
                <div class="card-description">{{ tool.description }}</div>

                @switch (tool.state) {
                  @case ('configuring') {
                    @if (tool.id === 'crossplane') {
                      <div class="config-section">
                        <div class="config-field">
                          <label>Crossplane Version</label>
                          <fd-select [value]="crossplaneSelectedVersion()" (valueChange)="crossplaneSelectedVersion.set($event)"
                            placeholder="Select version">
                            @for (v of crossplaneCatalog()?.spec?.versions ?? crossplaneDefaultVersions; track v.version) {
                              <fd-option [value]="v.version">{{ v.version }}</fd-option>
                            }
                          </fd-select>
                        </div>
                        @for (provider of crossplaneCatalog()?.spec?.providers ?? crossplaneDefaultProviders; track provider.name) {
                          <div class="config-field">
                            <label>{{ provider.name }}</label>
                            <fd-select [value]="crossplaneSelectedProviderVersions()[provider.name]"
                              (valueChange)="onCrossplaneProviderVersionChange(provider.name, $event)"
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
                        <div class="config-field">
                          <label>{{ tool.name }} Version</label>
                          <fd-select [value]="selectedVersions()[tool.id]"
                            (valueChange)="onToolVersionChange(tool.id, $event)"
                            placeholder="Select version">
                            @for (v of getVersions(tool.id); track v.version) {
                              <fd-option [value]="v.version">{{ v.version }}</fd-option>
                            }
                          </fd-select>
                        </div>
                      </div>
                    }
                  }
                  @case ('creating') {
                    <div class="inline-loading">
                      <fd-busy-indicator [loading]="true" size="m"></fd-busy-indicator>
                      <span>Installing {{ tool.name }}...</span>
                    </div>
                  }
                  @case ('provisioning') {
                    <div class="provisioning-status">
                      <fd-busy-indicator [loading]="true" size="s"></fd-busy-indicator>
                      <div>
                        <span>{{ tool.name }} is being set up...</span>
                        @if (getToolPhase(tool.id); as phase) {
                          <div class="phase-label" style="margin-top: 0.25rem">Phase: {{ phase }}</div>
                        }
                      </div>
                    </div>
                  }
                  @case ('active') {
                    <fd-message-strip type="success" [dismissible]="false">
                      {{ tool.name }} is installed and running in your workspace.
                    </fd-message-strip>
                  }
                }
              }
            </div>
            @if (drawerTool()?.state === 'configuring') {
              <div class="drawer-footer">
                @if (drawerToolId() === 'crossplane') {
                  <button fd-button label="Confirm and Install" fdType="emphasized"
                    [disabled]="!crossplaneSelectedVersion()"
                    (click)="onInstallCrossplane()"></button>
                } @else {
                  <button fd-button [label]="'Install ' + (drawerTool()?.name ?? '')" fdType="emphasized"
                    [disabled]="!selectedVersions()[drawerToolId()!]"
                    (click)="onInstallTool(drawerToolId()!)"></button>
                }
                <button fd-button label="Cancel" fdType="transparent"
                  (click)="closeDrawer()"></button>
              </div>
            }
          </div>
        }

        @if (binding() && (pendingClaims().length > 0 || acceptedClaims().length > 0)) {
          <div class="features-header" style="margin-top: 2rem;">
            <h2>Permission Claims</h2>
            <p>The OpenMCP services request access to the following resources in your workspace.</p>
          </div>
          <div class="onboarding-card">
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
export class FeaturesComponent implements OnDestroy {
  private crossplaneService = inject(CrossplaneOnboardingService);
  private kroService = inject(KROOnboardingService);
  private fluxService = inject(FluxOnboardingService);
  private ocmService = inject(OCMOnboardingService);

  private bindingWatchSub?: Subscription;
  private toolWatchSubs = new Map<string, Subscription>();
  private luigiContext!: LuigiContext;

  state = signal<FeaturesState>('loading');
  error = signal('');

  // Drawer state
  drawerToolId = signal<string | null>(null);
  drawerOpen = signal(false);

  // APIBinding state
  binding = signal<APIBindingDetail | null>(null);
  pendingClaims = signal<PermissionClaim[]>([]);
  acceptedClaims = signal<AcceptablePermissionClaim[]>([]);
  approvingClaim = signal('');

  // Tool status tracking
  crossplaneStatus = signal<CrossplaneStatus | null>(null);
  kroStatus = signal<KROStatus | null>(null);
  fluxStatus = signal<FluxStatus | null>(null);
  ocmStatus = signal<OCMControllerStatus | null>(null);

  // Crossplane-specific config
  crossplaneCatalog = signal<CrossplaneCatalog | null>(null);
  crossplaneSelectedVersion = signal('v1.20.1');
  crossplaneSelectedProviderVersions = signal<Record<string, string>>({ 'provider-kubernetes': 'v0.15.0' });
  readonly crossplaneDefaultVersions = [{ version: 'v1.20.1' }];
  readonly crossplaneDefaultProviders = [{ name: 'provider-kubernetes', versions: ['v0.15.0'] }];

  // Generic tool version selection
  selectedVersions = signal<Record<string, string>>({
    kro: 'v0.8.5',
    flux: 'v2.4.0',
    'ocm-controller': 'v0.29.0',
  });

  private readonly toolVersions: Record<string, VersionEntry[]> = {
    kro: [{ version: 'v0.8.5', chartVersion: '0.8.5' }],
    flux: [{ version: 'v2.4.0', chartVersion: '2.14.0' }],
    'ocm-controller': [{ version: 'v0.29.0', chartVersion: '0.0.0-6205a8a' }],
  };

  tools = signal<ToolCard[]>([
    { id: 'crossplane', name: 'Crossplane', icon: 'cloud', logoUrl: 'https://raw.githubusercontent.com/cncf/artwork/main/projects/crossplane/icon/color/crossplane-icon-color.svg', description: 'Provision and manage cloud infrastructure using Kubernetes-native APIs and resource compositions.', state: 'not-enabled' },
    { id: 'kro', name: 'KRO', icon: 'developer-settings', textLogo: 'kro', description: 'Define and manage custom resource compositions in your workspace with Kube Resource Orchestrator.', state: 'not-enabled' },
    { id: 'flux', name: 'Flux', icon: 'source-code', logoUrl: 'https://raw.githubusercontent.com/cncf/artwork/main/projects/flux/icon/color/flux-icon-color.svg', description: 'Continuous delivery with GitOps — keep your cluster state in sync with your Git repositories.', state: 'not-enabled' },
    { id: 'ocm-controller', name: 'OCM Controller', icon: 'cargo-train', logoUrl: 'https://ocm.software/svg/ocm-logo-icon-colour.svg', description: 'Manage software components and their delivery using Open Component Model.', state: 'not-enabled' },
  ]);

  @Input()
  LuigiClient!: LuigiClient;

  private luigiClient!: LuigiClient;

  @Input()
  set context(ctx: LuigiContext) {
    this.luigiClient = this.LuigiClient;
    this.luigiContext = ctx;
    this.crossplaneService.initialize(ctx);
    this.kroService.initialize(ctx);
    this.fluxService.initialize(ctx);
    this.ocmService.initialize(ctx);
    this.checkAPIBinding();
  }

  ngOnDestroy(): void {
    this.bindingWatchSub?.unsubscribe();
    this.toolWatchSubs.forEach((sub) => sub.unsubscribe());
  }

  drawerTool(): ToolCard | null {
    const id = this.drawerToolId();
    if (!id) return null;
    return this.tools().find((t) => t.id === id) ?? null;
  }

  // --- APIBinding activation ---

  onActivate(): void {
    this.state.set('activating');
    this.error.set('');
    this.crossplaneService.createAPIBinding().subscribe({
      next: () => this.pollAPIBindingReady(),
      error: (err: Error) => {
        this.error.set(`Failed to activate OpenMCP services: ${err.message}`);
        this.state.set('activate');
      },
    });
  }

  onAcceptClaim(claim: PermissionClaim): void {
    const currentBinding = this.binding();
    if (!currentBinding) return;

    this.approvingClaim.set(claim.resource + claim.group);
    this.error.set('');
    this.crossplaneService
      .acceptPermissionClaim(currentBinding, claim)
      .subscribe({
        next: () => this.approvingClaim.set(''),
        error: (err: Error) => {
          this.error.set(`Failed to accept permission claim: ${err.message}`);
          this.approvingClaim.set('');
        },
      });
  }

  // --- Tool enable/install ---

  onEnableTool(toolId: string): void {
    this.updateToolState(toolId, 'configuring');
    this.drawerToolId.set(toolId);
    // Trigger open on next tick so the enter animation plays
    setTimeout(() => this.drawerOpen.set(true), 10);
    if (toolId === 'crossplane') {
      this.loadCrossplaneCatalog();
    }
  }

  closeDrawer(): void {
    this.drawerOpen.set(false);
    const toolId = this.drawerToolId();
    // After animation, clear the drawer and revert configuring state if needed
    setTimeout(() => {
      if (toolId) {
        const tool = this.tools().find((t) => t.id === toolId);
        if (tool?.state === 'configuring') {
          this.updateToolState(toolId, 'not-enabled');
        }
      }
      this.drawerToolId.set(null);
    }, 250);
  }

  onCrossplaneProviderVersionChange(providerName: string, version: string): void {
    this.crossplaneSelectedProviderVersions.update((prev) => ({
      ...prev,
      [providerName]: version,
    }));
  }

  onToolVersionChange(toolId: string, version: string): void {
    this.selectedVersions.update((prev) => ({ ...prev, [toolId]: version }));
  }

  getVersions(toolId: string): VersionEntry[] {
    return this.toolVersions[toolId] ?? [];
  }

  getToolPhase(toolId: string): string {
    switch (toolId) {
      case 'crossplane': return this.crossplaneStatus()?.status?.phase ?? '';
      case 'kro': return this.kroStatus()?.status?.phase ?? '';
      case 'flux': return this.fluxStatus()?.status?.phase ?? '';
      case 'ocm-controller': return this.ocmStatus()?.status?.phase ?? '';
      default: return '';
    }
  }

  onInstallCrossplane(): void {
    this.updateToolState('crossplane', 'creating');
    this.error.set('');
    const providers = Object.entries(this.crossplaneSelectedProviderVersions())
      .filter(([, version]) => !!version)
      .map(([name, version]) => ({ name, version }));
    this.crossplaneService
      .createCrossplane(this.crossplaneSelectedVersion(), providers)
      .subscribe({
        next: () => this.startWatchingTool('crossplane'),
        error: (err: Error) => {
          this.error.set(`Failed to install Crossplane: ${err.message}`);
          this.updateToolState('crossplane', 'configuring');
        },
      });
  }

  onInstallTool(toolId: string): void {
    this.updateToolState(toolId, 'creating');
    this.error.set('');
    const version = this.selectedVersions()[toolId];
    const entry = this.toolVersions[toolId]?.find((v) => v.version === version);
    const chartVersion = entry?.chartVersion;

    let create$;
    switch (toolId) {
      case 'kro':
        create$ = this.kroService.createKRO(version, chartVersion);
        break;
      case 'flux':
        create$ = this.fluxService.createFlux(version, chartVersion);
        break;
      case 'ocm-controller':
        create$ = this.ocmService.createOCMController(version, chartVersion);
        break;
      default:
        return;
    }

    create$.subscribe({
      next: () => this.startWatchingTool(toolId),
      error: (err: Error) => {
        this.error.set(`Failed to install ${toolId}: ${err.message}`);
        this.updateToolState(toolId, 'configuring');
      },
    });
  }

  onDisableTool(toolId: string): void {
    this.updateToolState(toolId, 'disabling');
    this.error.set('');

    let delete$: Observable<void>;
    switch (toolId) {
      case 'crossplane':
        delete$ = this.crossplaneService.deleteCrossplane();
        break;
      case 'kro':
        delete$ = this.kroService.deleteKRO();
        break;
      case 'flux':
        delete$ = this.fluxService.deleteFlux();
        break;
      case 'ocm-controller':
        delete$ = this.ocmService.deleteOCMController();
        break;
      default:
        return;
    }

    delete$.subscribe({
      next: () => {
        this.setToolStatus(toolId, {});
        this.updateToolState(toolId, 'not-enabled');
        this.sendPortalReloadMessage(toolId);
      },
      error: (err: Error) => {
        this.error.set(`Failed to disable ${toolId}: ${err.message}`);
        this.updateToolState(toolId, 'active');
      },
    });
  }

  // --- Private methods ---

  private checkAPIBinding(): void {
    this.state.set('loading');
    this.crossplaneService.checkAPIBinding().subscribe({
      next: (binding) => {
        if (!binding) {
          this.state.set('activate');
        } else {
          this.binding.set(binding);
          this.computeClaims(binding);
          this.startWatchingBinding();
          this.state.set('features');
          this.checkAllStatuses();
        }
      },
      error: (err: Error) => {
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
    this.bindingWatchSub = this.crossplaneService
      .watchAPIBinding()
      .subscribe({
        next: (event) => {
          this.binding.set(event.object);
          this.computeClaims(event.object);
        },
        error: () => {},
      });
  }

  private pollAPIBindingReady(): void {
    this.crossplaneService.checkAPIBinding().subscribe({
      next: (binding) => {
        if (binding?.status?.phase === 'Bound') {
          this.binding.set(binding);
          this.computeClaims(binding);
          this.startWatchingBinding();
          this.state.set('features');
          this.checkAllStatuses();
        } else {
          setTimeout(() => this.pollAPIBindingReady(), 2000);
        }
      },
      error: () => {
        setTimeout(() => this.pollAPIBindingReady(), 2000);
      },
    });
  }

  private checkAllStatuses(): void {
    forkJoin({
      crossplane: this.crossplaneService.checkCrossplane(),
      kro: this.kroService.checkKRO(),
      flux: this.fluxService.checkFlux(),
      ocm: this.ocmService.checkOCMController(),
    }).subscribe({
      next: (results) => {
        const updated = [...this.tools()];

        if (results.crossplane?.status?.phase === 'Ready') {
          this.crossplaneStatus.set(results.crossplane);
          updated[0] = { ...updated[0], state: 'active' };
        } else if (results.crossplane) {
          this.crossplaneStatus.set(results.crossplane);
          updated[0] = { ...updated[0], state: 'provisioning' };
          this.startWatchingTool('crossplane');
        }

        if (results.kro?.status?.phase === 'Ready') {
          this.kroStatus.set(results.kro);
          updated[1] = { ...updated[1], state: 'active' };
        } else if (results.kro) {
          this.kroStatus.set(results.kro);
          updated[1] = { ...updated[1], state: 'provisioning' };
          this.startWatchingTool('kro');
        }

        if (results.flux?.status?.phase === 'Ready') {
          this.fluxStatus.set(results.flux);
          updated[2] = { ...updated[2], state: 'active' };
        } else if (results.flux) {
          this.fluxStatus.set(results.flux);
          updated[2] = { ...updated[2], state: 'provisioning' };
          this.startWatchingTool('flux');
        }

        if (results.ocm?.status?.phase === 'Ready') {
          this.ocmStatus.set(results.ocm);
          updated[3] = { ...updated[3], state: 'active' };
        } else if (results.ocm) {
          this.ocmStatus.set(results.ocm);
          updated[3] = { ...updated[3], state: 'provisioning' };
          this.startWatchingTool('ocm-controller');
        }

        this.tools.set(updated);
      },
      error: () => {},
    });
  }

  private loadCrossplaneCatalog(): void {
    this.crossplaneService.getCatalog().subscribe({
      next: (catalog) => {
        if (catalog) {
          this.crossplaneCatalog.set(catalog);
          this.crossplaneSelectedVersion.set(catalog.spec.versions[0]?.version ?? '');
          const providerDefaults: Record<string, string> = {};
          for (const provider of catalog.spec.providers) {
            if (provider.versions.length > 0) {
              providerDefaults[provider.name] = provider.versions[0];
            }
          }
          this.crossplaneSelectedProviderVersions.set(providerDefaults);
        }
      },
      error: () => {},
    });
  }

  private getToolWatch(toolId: string): Observable<ToolWatchEvent> | null {
    switch (toolId) {
      case 'crossplane':
        return this.crossplaneService.watchCrossplane().pipe(
          map((e) => ({ object: e.object })),
        );
      case 'kro':
        return this.kroService.watchKRO().pipe(
          map((e) => ({ object: e.object })),
        );
      case 'flux':
        return this.fluxService.watchFlux().pipe(
          map((e) => ({ object: e.object })),
        );
      case 'ocm-controller':
        return this.ocmService.watchOCMController().pipe(
          map((e) => ({ object: e.object })),
        );
      default:
        return null;
    }
  }

  private startWatchingTool(toolId: string): void {
    this.updateToolState(toolId, 'provisioning');
    const existing = this.toolWatchSubs.get(toolId);
    existing?.unsubscribe();

    const watch$ = this.getToolWatch(toolId);
    if (!watch$) return;

    const sub = watch$.subscribe({
      next: (event) => {
        this.setToolStatus(toolId, event.object);
        if (event.object.status?.phase === 'Ready') {
          this.updateToolState(toolId, 'active');
          this.sendPortalReloadMessage(toolId);
          sub.unsubscribe();
          this.toolWatchSubs.delete(toolId);
          // Close drawer if it's showing this tool
          if (this.drawerToolId() === toolId) {
            setTimeout(() => {
              this.drawerOpen.set(false);
              setTimeout(() => this.drawerToolId.set(null), 250);
            }, 1500);
          }
        }
      },
      error: (err: Error) => {
        this.error.set(`Watch error for ${toolId}: ${err.message}`);
      },
    });
    this.toolWatchSubs.set(toolId, sub);
  }

  private setToolStatus(toolId: string, obj: ToolWatchEvent['object']): void {
    switch (toolId) {
      case 'crossplane': this.crossplaneStatus.set(obj as CrossplaneStatus); break;
      case 'kro': this.kroStatus.set(obj as KROStatus); break;
      case 'flux': this.fluxStatus.set(obj as FluxStatus); break;
      case 'ocm-controller': this.ocmStatus.set(obj as OCMControllerStatus); break;
    }
  }

  private updateToolState(toolId: string, newState: ToolState): void {
    this.tools.update((prev) =>
      prev.map((t) => (t.id === toolId ? { ...t, state: newState } : t)),
    );
  }

  private sendPortalReloadMessage(toolId: string): void {
    const entityType = this.luigiContext?.entityType ?? '';
    sendCustomMessage({
      id: 'openmfp.reload-luigi-config',
      origin: 'FeaturesOnboarding',
      action: `provision-${toolId}`,
      entity: entityType,
      context: {
        [entityType]: this.luigiContext?.entityName,
        user: this.luigiContext?.userId,
      },
    });
  }
}
