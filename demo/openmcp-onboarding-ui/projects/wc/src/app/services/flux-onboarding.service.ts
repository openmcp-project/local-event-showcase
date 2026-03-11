import { ApolloFactory, LuigiContext } from './apollo-factory';
import { Injectable, inject } from '@angular/core';
import { Apollo } from 'apollo-angular';
import { Observable, map, of, catchError } from 'rxjs';
import { gql } from '@apollo/client/core';

export interface FluxStatus {
  metadata: { name: string };
  spec?: { version: string; chartVersion?: string; components?: string[] };
  status?: { phase: string };
}

export interface FluxEvent {
  type: 'ADDED' | 'MODIFIED' | 'DELETED';
  object: FluxStatus;
}

const CHECK_FLUX = gql`
  query {
    flux_services_openmcp_cloud {
      v1alpha1 {
        Flux(name: "default") {
          metadata {
            name
          }
          spec {
            version
            components
          }
          status {
            phase
          }
        }
      }
    }
  }
`;

const WATCH_FLUX = gql`
  subscription {
    flux_services_openmcp_cloud_v1alpha1_flux(name: "default") {
      type
      object {
        metadata {
          name
        }
        spec {
          version
          components
        }
        status {
          phase
        }
      }
    }
  }
`;

@Injectable({
  providedIn: 'root',
})
export class FluxOnboardingService {
  private apolloFactory = inject(ApolloFactory);
  private apollo!: Apollo;

  public initialize(context: LuigiContext): void {
    this.apollo = this.apolloFactory.apollo(context);
  }

  public checkFlux(): Observable<FluxStatus | null> {
    return this.apollo
      .query<{
        flux_services_openmcp_cloud: {
          v1alpha1: { Flux: FluxStatus | null };
        };
      }>({
        query: CHECK_FLUX,
        fetchPolicy: 'network-only',
      })
      .pipe(
        map((result) => result.data!.flux_services_openmcp_cloud.v1alpha1.Flux),
        catchError((err) => {
          if (err.message?.includes('not found')) {
            return of(null);
          }
          throw err;
        }),
      );
  }

  public createFlux(
    version: string,
    chartVersion?: string,
  ): Observable<{ metadata: { name: string } }> {
    const chartVersionInput = chartVersion ? `chartVersion: "${chartVersion}"` : '';
    const mutation = gql`
      mutation {
        flux_services_openmcp_cloud {
          v1alpha1 {
            createFlux(
              object: {
                metadata: { name: "default" }
                spec: {
                  version: "${version}"
                  ${chartVersionInput}
                }
              }
            ) {
              metadata {
                name
              }
            }
          }
        }
      }
    `;
    return this.apollo
      .mutate<{
        flux_services_openmcp_cloud: {
          v1alpha1: {
            createFlux: { metadata: { name: string } };
          };
        };
      }>({
        mutation,
      })
      .pipe(
        map((result) => result.data!.flux_services_openmcp_cloud.v1alpha1.createFlux),
      );
  }

  public watchFlux(): Observable<FluxEvent> {
    return this.apollo
      .subscribe<{
        flux_services_openmcp_cloud_v1alpha1_flux: FluxEvent;
      }>({
        query: WATCH_FLUX,
      })
      .pipe(
        map((result) => result.data!.flux_services_openmcp_cloud_v1alpha1_flux),
      );
  }
}
