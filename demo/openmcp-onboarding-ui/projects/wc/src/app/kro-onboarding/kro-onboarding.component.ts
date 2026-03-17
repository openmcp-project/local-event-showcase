import {
  KROOnboardingService,
  KROStatus,
} from '../services/kro-onboarding.service';
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
  | 'configure'
  | 'creating'
  | 'provisioning'
  | 'active';

@Component({
  selector: 'app-kro-onboarding',
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
    @switch (state()) {
      @case ('loading') {
        <div class="loading-container">
          <fd-busy-indicator [loading]="true" size="m"></fd-busy-indicator>
          <span>Checking KRO status...</span>
        </div>
      }

      @case ('configure') {
        <div class="onboarding-card">
          <div class="card-header">
            <fd-icon glyph="developer-settings"></fd-icon>
            <h2>Enable KRO</h2>
          </div>
          <div class="card-description">
            KRO (Kube Resource Orchestrator) enables you to define and manage
            custom resource compositions in your workspace. Select a version
            to get started.
          </div>
          <div class="config-section">
            <div class="config-field">
              <label>KRO Version</label>
              <fd-select [value]="selectedVersion()" (valueChange)="onVersionChange($event)"
                placeholder="Select version">
                @for (v of availableVersions; track v.version) {
                  <fd-option [value]="v.version">{{ v.version }}</fd-option>
                }
              </fd-select>
            </div>
          </div>
          <button fd-button label="Install KRO" fdType="emphasized"
            [disabled]="!selectedVersion()"
            (click)="onConfigure()"></button>
        </div>
      }

      @case ('creating') {
        <div class="loading-container">
          <fd-busy-indicator [loading]="true" size="m"></fd-busy-indicator>
          <span>Installing KRO...</span>
        </div>
      }

      @case ('provisioning') {
        <div class="provisioning-card">
          <div class="provisioning-header">
            <fd-busy-indicator [loading]="true" size="s"></fd-busy-indicator>
            <h2>KRO is provisioning</h2>
          </div>
          <div class="provisioning-status">
            <div>
              <div class="phase-text">Your KRO instance is being set up. This may take a few minutes.</div>
              @if (kro()?.status?.phase) {
                <div class="phase-label" style="margin-top: 0.5rem">
                  Phase: {{ kro()!.status!.phase }}
                </div>
              }
            </div>
          </div>
          @if (kro()) {
            <div class="config-section">
              <div class="config-row">
                <span class="config-label">Version</span>
                <span class="config-value">{{ kro()!.spec?.version }}</span>
              </div>
              <div class="config-row">
                <span class="config-label">Status</span>
                <span class="config-value">
                  <span class="status-badge provisioning">{{ kro()!.status?.phase ?? 'Pending' }}</span>
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
            <h2>KRO Active</h2>
          </div>
          @if (kro()) {
            <div class="config-section">
              <div class="config-row">
                <span class="config-label">Version</span>
                <span class="config-value">{{ kro()!.spec?.version }}</span>
              </div>
              <div class="config-row">
                <span class="config-label">Status</span>
                <span class="config-value">
                  <span class="status-badge">{{ kro()!.status?.phase ?? 'Unknown' }}</span>
                </span>
              </div>
            </div>
          }
          <fd-message-strip type="success" [dismissible]="false">
            KRO is installed and running in your workspace.
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
export class KROOnboardingComponent implements OnDestroy {
  private onboardingService = inject(KROOnboardingService);
  private watchSub?: Subscription;
  private luigiContext!: LuigiContext;

  readonly availableVersions = [{ version: 'v0.8.5', chartVersion: '0.8.5' }];

  state = signal<OnboardingState>('loading');
  error = signal('');
  kro = signal<KROStatus | null>(null);
  selectedVersion = signal('v0.8.5');
  selectedChartVersion = signal('0.8.5');

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
  }

  onVersionChange(version: string): void {
    this.selectedVersion.set(version);
    const entry = this.availableVersions.find((v) => v.version === version);
    this.selectedChartVersion.set(entry?.chartVersion ?? '');
  }

  onConfigure(): void {
    this.state.set('creating');
    this.error.set('');
    this.onboardingService.createKRO(this.selectedVersion(), this.selectedChartVersion() || undefined).subscribe({
      next: () => this.startWatching(),
      error: (err) => {
        this.error.set(`Failed to create KRO: ${err.message}`);
        this.state.set('configure');
      },
    });
  }

  private checkState(): void {
    this.state.set('loading');
    this.onboardingService.checkKRO().subscribe({
      next: (resource) => {
        if (resource) {
          this.kro.set(resource);
          if (resource.status?.phase === 'Ready') {
            this.state.set('active');
          } else {
            this.startWatching();
          }
        } else {
          this.state.set('configure');
        }
      },
      error: () => {
        this.state.set('configure');
      },
    });
  }

  private startWatching(): void {
    this.state.set('provisioning');
    this.watchSub?.unsubscribe();
    this.watchSub = this.onboardingService.watchKRO().subscribe({
      next: (event) => {
        this.kro.set(event.object);
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
      origin: 'KROOnboarding',
      action: 'provisionKRO',
      entity: entityType,
      context: {
        [entityType]: this.luigiContext?.entityName,
        user: this.luigiContext?.userId,
      },
    });
  }
}
