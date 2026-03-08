import { ApolloFactory, LuigiContext } from './apollo-factory';
import { Injectable, inject } from '@angular/core';
import { Apollo } from 'apollo-angular';
import { Observable, map, of, catchError } from 'rxjs';
import { gql } from '@apollo/client/core';

export interface APIBindingStatus {
  metadata: { name: string };
  status?: { phase: string };
}

export interface CrossplaneStatus {
  metadata: { name: string };
  spec?: { version: string; providers: { name: string; version: string }[] };
  status?: { phase: string };
}

export interface CrossplaneEvent {
  type: 'ADDED' | 'MODIFIED' | 'DELETED';
  object: CrossplaneStatus;
}

const CHECK_API_BINDING = gql`
  query {
    apis_kcp_io {
      v1alpha2 {
        APIBinding(name: "crossplane.services.openmcp.cloud") {
          metadata {
            name
          }
          status {
            phase
          }
        }
      }
    }
  }
`;

const CREATE_API_BINDING = gql`
  mutation {
    apis_kcp_io {
      v1alpha2 {
        createAPIBinding(
          object: {
            metadata: { name: "crossplane.services.openmcp.cloud" }
            spec: {
              reference: {
                export: {
                  name: "crossplane.services.openmcp.cloud"
                }
              }
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

const CHECK_CROSSPLANE = gql`
  query {
    crossplane_services_openmcp_cloud {
      v1alpha1 {
        Crossplane(name: "default") {
          metadata {
            name
          }
          spec {
            version
            providers {
              name
              version
            }
          }
          status {
            phase
          }
        }
      }
    }
  }
`;

const WATCH_CROSSPLANE = gql`
  subscription {
    crossplane_services_openmcp_cloud_v1alpha1_crossplane(name: "default") {
      type
      object {
        metadata {
          name
        }
        spec {
          version
          providers {
            name
            version
          }
        }
        status {
          phase
        }
      }
    }
  }
`;

const CREATE_CROSSPLANE = gql`
  mutation {
    crossplane_services_openmcp_cloud {
      v1alpha1 {
        createCrossplane(
          object: {
            metadata: { name: "default" }
            spec: {
              version: "v1.20.1"
              providers: [
                { name: "provider-kubernetes", version: "v0.15.0" }
              ]
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

@Injectable({
  providedIn: 'root',
})
export class CrossplaneOnboardingService {
  private apolloFactory = inject(ApolloFactory);
  private apollo!: Apollo;

  public initialize(context: LuigiContext): void {
    this.apollo = this.apolloFactory.apollo(context);
  }

  public checkAPIBinding(): Observable<APIBindingStatus | null> {
    return this.apollo
      .query<{
        apis_kcp_io: {
          v1alpha2: { APIBinding: APIBindingStatus | null };
        };
      }>({
        query: CHECK_API_BINDING,
        fetchPolicy: 'network-only',
      })
      .pipe(
        map((result) => result.data!.apis_kcp_io.v1alpha2.APIBinding),
        catchError((err) => {
          if (err.message?.includes('not found')) {
            return of(null);
          }
          throw err;
        }),
      );
  }

  public createAPIBinding(): Observable<{ metadata: { name: string } }> {
    return this.apollo
      .mutate<{
        apis_kcp_io: {
          v1alpha2: {
            createAPIBinding: { metadata: { name: string } };
          };
        };
      }>({
        mutation: CREATE_API_BINDING,
      })
      .pipe(
        map((result) => result.data!.apis_kcp_io.v1alpha2.createAPIBinding),
      );
  }

  public checkCrossplane(): Observable<CrossplaneStatus | null> {
    return this.apollo
      .query<{
        crossplane_services_openmcp_cloud: {
          v1alpha1: { Crossplane: CrossplaneStatus | null };
        };
      }>({
        query: CHECK_CROSSPLANE,
        fetchPolicy: 'network-only',
      })
      .pipe(
        map((result) => result.data!.crossplane_services_openmcp_cloud.v1alpha1.Crossplane),
        catchError((err) => {
          if (err.message?.includes('not found')) {
            return of(null);
          }
          throw err;
        }),
      );
  }

  public createCrossplane(): Observable<{ metadata: { name: string } }> {
    return this.apollo
      .mutate<{
        crossplane_services_openmcp_cloud: {
          v1alpha1: {
            createCrossplane: { metadata: { name: string } };
          };
        };
      }>({
        mutation: CREATE_CROSSPLANE,
      })
      .pipe(
        map((result) => result.data!.crossplane_services_openmcp_cloud.v1alpha1.createCrossplane),
      );
  }

  public watchCrossplane(): Observable<CrossplaneEvent> {
    return this.apollo
      .subscribe<{
        crossplane_services_openmcp_cloud_v1alpha1_crossplane: CrossplaneEvent;
      }>({
        query: WATCH_CROSSPLANE,
      })
      .pipe(
        map((result) => result.data!.crossplane_services_openmcp_cloud_v1alpha1_crossplane),
      );
  }
}
