import {
  CrossplaneOnboardingService,
  CrossplaneStatus,
} from '../services/crossplane-onboarding.service';
import { LuigiContext } from '../services/apollo-factory';
import {
  ChangeDetectionStrategy,
  Component,
  Input,
  ViewEncapsulation,
  inject,
  signal,
} from '@angular/core';
import { LuigiClient } from '@luigi-project/client/luigi-element';
import { ButtonComponent } from '@fundamental-ngx/core/button';
import { BusyIndicatorComponent } from '@fundamental-ngx/core/busy-indicator';
import { MessageStripComponent } from '@fundamental-ngx/core/message-strip';
import { IconComponent } from '@fundamental-ngx/core/icon';
import { ContentDensityDirective } from '@fundamental-ngx/core/content-density';

type OnboardingState =
  | 'loading'
  | 'activate'
  | 'activating'
  | 'configure'
  | 'creating'
  | 'active';

@Component({
  selector: 'app-crossplane-onboarding',
  standalone: true,
  imports: [
    ButtonComponent,
    BusyIndicatorComponent,
    MessageStripComponent,
    IconComponent,
    ContentDensityDirective,
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
      padding: 0.5rem 0;
    }

    .config-row + .config-row {
      border-top: 1px solid var(--sapList_BorderColor, #e4e4e4);
    }

    .config-label {
      color: var(--sapContent_LabelColor, #6a6d70);
      font-size: var(--sapFontSmallSize, 0.75rem);
    }

    .config-value {
      font-weight: bold;
      color: var(--sapTextColor, #32363a);
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
  `,
  template: `
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
          <div class="config-section">
            <div class="config-row">
              <span class="config-label">Crossplane Version</span>
              <span class="config-value">v1.20.1</span>
            </div>
            <div class="config-row">
              <span class="config-label">Provider</span>
              <span class="config-value">provider-kubernetes v0.15.0</span>
            </div>
          </div>
          <button fd-button label="Confirm and Install" fdType="emphasized"
            (click)="onConfigure()"></button>
        </div>
      }

      @case ('creating') {
        <div class="loading-container">
          <fd-busy-indicator [loading]="true" size="m"></fd-busy-indicator>
          <span>Installing Crossplane...</span>
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
export class CrossplaneOnboardingComponent {
  private onboardingService = inject(CrossplaneOnboardingService);

  state = signal<OnboardingState>('loading');
  error = signal('');
  crossplane = signal<CrossplaneStatus | null>(null);

  @Input()
  LuigiClient!: LuigiClient;

  @Input()
  set context(ctx: LuigiContext) {
    this.onboardingService.initialize(ctx);
    this.checkState();
  }

  onActivate(): void {
    this.state.set('activating');
    this.error.set('');
    this.onboardingService.createAPIBinding().subscribe({
      next: () => this.pollAPIBindingReady(),
      error: (err) => {
        this.error.set(`Failed to create API binding: ${err.message}`);
        this.state.set('activate');
      },
    });
  }

  onConfigure(): void {
    this.state.set('creating');
    this.error.set('');
    this.onboardingService.createCrossplane().subscribe({
      next: () => this.checkCrossplaneState(),
      error: (err) => {
        this.error.set(`Failed to create Crossplane: ${err.message}`);
        this.state.set('configure');
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
          this.checkCrossplaneState();
        }
      },
      error: (err) => {
        this.error.set(`Failed to check API binding: ${err.message}`);
        this.state.set('activate');
      },
    });
  }

  private checkCrossplaneState(): void {
    this.onboardingService.checkCrossplane().subscribe({
      next: (cp) => {
        if (cp) {
          this.crossplane.set(cp);
          this.state.set('active');
        } else {
          this.state.set('configure');
        }
      },
      error: () => {
        this.state.set('configure');
      },
    });
  }

  private pollAPIBindingReady(): void {
    this.onboardingService.checkAPIBinding().subscribe({
      next: (binding) => {
        if (binding?.status?.phase === 'Bound') {
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
