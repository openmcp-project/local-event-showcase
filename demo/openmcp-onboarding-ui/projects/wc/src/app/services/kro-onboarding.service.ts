import { ApolloFactory, LuigiContext } from './apollo-factory';
import { Injectable, inject } from '@angular/core';
import { Apollo } from 'apollo-angular';
import { Observable, map, of, catchError } from 'rxjs';
import { gql } from '@apollo/client/core';

export interface KROStatus {
  metadata: { name: string };
  spec?: { version: string; chartVersion?: string };
  status?: { phase: string };
}

export interface KROEvent {
  type: 'ADDED' | 'MODIFIED' | 'DELETED';
  object: KROStatus;
}

const CHECK_KRO = gql`
  query {
    kro_services_opencp_cloud {
      v1alpha1 {
        KRO(name: "default") {
          metadata {
            name
          }
          spec {
            version
          }
          status {
            phase
          }
        }
      }
    }
  }
`;

const WATCH_KRO = gql`
  subscription {
    kro_services_opencp_cloud_v1alpha1_kro(name: "default") {
      type
      object {
        metadata {
          name
        }
        spec {
          version
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
export class KROOnboardingService {
  private apolloFactory = inject(ApolloFactory);
  private apollo!: Apollo;

  public initialize(context: LuigiContext): void {
    this.apollo = this.apolloFactory.apollo(context);
  }

  public checkKRO(): Observable<KROStatus | null> {
    return this.apollo
      .query<{
        kro_services_opencp_cloud: {
          v1alpha1: { KRO: KROStatus | null };
        };
      }>({
        query: CHECK_KRO,
        fetchPolicy: 'network-only',
      })
      .pipe(
        map((result) => result.data!.kro_services_opencp_cloud.v1alpha1.KRO),
        catchError((err) => {
          if (err.message?.includes('not found')) {
            return of(null);
          }
          throw err;
        }),
      );
  }

  public createKRO(version: string, chartVersion?: string): Observable<{ metadata: { name: string } }> {
    const chartVersionInput = chartVersion ? `chartVersion: "${chartVersion}"` : '';
    const mutation = gql`
      mutation {
        kro_services_opencp_cloud {
          v1alpha1 {
            createKRO(
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
        kro_services_opencp_cloud: {
          v1alpha1: {
            createKRO: { metadata: { name: string } };
          };
        };
      }>({
        mutation,
      })
      .pipe(
        map((result) => result.data!.kro_services_opencp_cloud.v1alpha1.createKRO),
      );
  }

  public watchKRO(): Observable<KROEvent> {
    return this.apollo
      .subscribe<{
        kro_services_opencp_cloud_v1alpha1_kro: KROEvent;
      }>({
        query: WATCH_KRO,
      })
      .pipe(
        map((result) => result.data!.kro_services_opencp_cloud_v1alpha1_kro),
      );
  }

  public deleteKRO(): Observable<void> {
    const mutation = gql`
      mutation {
        kro_services_opencp_cloud {
          v1alpha1 {
            deleteKRO(name: "default")
          }
        }
      }
    `;
    return this.apollo.mutate({ mutation }).pipe(map(() => void 0));
  }
}
