import { ApolloFactory, LuigiContext } from './apollo-factory';
import { Injectable, inject } from '@angular/core';
import { Apollo } from 'apollo-angular';
import { Observable, map, of, catchError, throwError } from 'rxjs';
import { gql } from '@apollo/client/core';

export interface PermissionClaim {
  group: string;
  resource: string;
  verbs: string[];
  identityHash: string;
}

export interface AcceptablePermissionClaim extends PermissionClaim {
  state: 'Accepted' | 'Rejected';
}

export interface APIBindingDetail {
  metadata: { name: string; resourceVersion: string };
  spec: {
    permissionClaims: AcceptablePermissionClaim[] | null;
    reference: { export: { name: string } };
  };
  status: {
    phase: string;
    exportPermissionClaims: PermissionClaim[] | null;
  };
}

export interface APIBindingEvent {
  type: 'ADDED' | 'MODIFIED' | 'DELETED';
  object: APIBindingDetail;
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

export interface CrossplaneCatalog {
  metadata: { name: string };
  spec: {
    versions: { version: string }[];
    providers: { name: string; versions: string[] }[];
  };
}

const CHECK_API_BINDING = gql`
  query {
    apis_kcp_io {
      v1alpha2 {
        APIBinding(name: "crossplane.services.openmcp.cloud") {
          metadata {
            name
            resourceVersion
          }
          spec {
            permissionClaims {
              group
              resource
              verbs
              identityHash
              state
            }
            reference {
              export {
                name
              }
            }
          }
          status {
            phase
            exportPermissionClaims {
              group
              resource
              verbs
              identityHash
            }
          }
        }
      }
    }
  }
`;

const LIST_API_BINDINGS = gql`
  query {
    apis_kcp_io {
      v1alpha2 {
        APIBindings {
          items {
            metadata {
              name
              resourceVersion
            }
            spec {
              permissionClaims {
                group
                resource
                verbs
                identityHash
                state
              }
              reference {
                export {
                  name
                }
              }
            }
            status {
              phase
              exportPermissionClaims {
                group
                resource
                verbs
                identityHash
              }
            }
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

const QUERY_CATALOG = gql`
  query {
    crossplane_services_openmcp_cloud {
      v1alpha1 {
        CrossplaneCatalog(name: "default") {
          metadata {
            name
          }
          spec {
            versions {
              version
            }
            providers {
              name
              versions
            }
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

  public checkAPIBinding(): Observable<APIBindingDetail | null> {
    return this.apollo
      .query<{
        apis_kcp_io: {
          v1alpha2: { APIBinding: APIBindingDetail | null };
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

  public listAPIBindings(): Observable<APIBindingDetail[]> {
    return this.apollo
      .query<{
        apis_kcp_io: {
          v1alpha2: { APIBindings: { items: APIBindingDetail[] } };
        };
      }>({
        query: LIST_API_BINDINGS,
        fetchPolicy: 'network-only',
      })
      .pipe(
        map((result) => result.data!.apis_kcp_io.v1alpha2.APIBindings.items ?? []),
        catchError(() => of([])),
      );
  }

  public watchAPIBinding(bindingName: string): Observable<APIBindingEvent> {
    const query = gql`
      subscription {
        apis_kcp_io_v1alpha2_apibinding(
          name: "${bindingName}"
          subscribeToAll: true
        ) {
          type
          object {
            metadata {
              name
              resourceVersion
            }
            spec {
              permissionClaims {
                group
                resource
                verbs
                identityHash
                state
              }
              reference {
                export {
                  name
                }
              }
            }
            status {
              phase
              exportPermissionClaims {
                group
                resource
                verbs
                identityHash
              }
            }
          }
        }
      }
    `;
    return this.apollo
      .subscribe<{
        apis_kcp_io_v1alpha2_apibinding: APIBindingEvent;
      }>({
        query,
      })
      .pipe(
        map(
          (result) => result.data!.apis_kcp_io_v1alpha2_apibinding,
        ),
      );
  }

  public createAPIBinding(exportName: string): Observable<{ metadata: { name: string } }> {
    const mutation = gql`
      mutation {
        apis_kcp_io {
          v1alpha2 {
            createAPIBinding(
              object: {
                metadata: { name: "${exportName}" }
                spec: {
                  reference: {
                    export: {
                      name: "${exportName}"
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
    return this.apollo
      .mutate<{
        apis_kcp_io: {
          v1alpha2: {
            createAPIBinding: { metadata: { name: string } };
          };
        };
      }>({
        mutation,
      })
      .pipe(
        map((result) => result.data!.apis_kcp_io.v1alpha2.createAPIBinding),
      );
  }

  public deleteAPIBinding(bindingName: string): Observable<void> {
    const mutation = gql`
      mutation {
        apis_kcp_io {
          v1alpha2 {
            deleteAPIBinding(name: "${bindingName}")
          }
        }
      }
    `;

    return this.apollo.mutate({ mutation }).pipe(
      map(() => void 0),
      catchError((err) => {
        if (err.message?.includes('not found')) {
          return of(void 0);
        }
        return throwError(() => err);
      }),
    );
  }

  public acceptPermissionClaim(
    bindingName: string,
    binding: APIBindingDetail,
    claim: PermissionClaim,
  ): Observable<{ metadata: { name: string; resourceVersion: string } }> {
    const existingClaims = (binding.spec.permissionClaims ?? []).filter(
      (c) => c.state === 'Accepted',
    );
    const allClaims = [
      ...existingClaims,
      { ...claim, state: 'Accepted' as const },
    ];

    const claimsInput = allClaims
      .map(
        (c) =>
          `{ group: "${c.group}", resource: "${c.resource}", verbs: [${c.verbs.map((v) => `"${v}"`).join(', ')}], identityHash: "${c.identityHash}", state: "Accepted", selector: { matchAll: true } }`,
      )
      .join(',\n              ');

    const mutation = gql`
      mutation {
        apis_kcp_io {
          v1alpha2 {
            updateAPIBinding(
              name: "${bindingName}"
              object: {
                metadata: { resourceVersion: "${binding.metadata.resourceVersion}" }
                spec: {
                  reference: { export: { name: "${binding.spec.reference.export.name}" } }
                  permissionClaims: [
                    ${claimsInput}
                  ]
                }
              }
            ) {
              metadata {
                name
                resourceVersion
              }
            }
          }
        }
      }
    `;

    return this.apollo
      .mutate<{
        apis_kcp_io: {
          v1alpha2: {
            updateAPIBinding: {
              metadata: { name: string; resourceVersion: string };
            };
          };
        };
      }>({
        mutation,
      })
      .pipe(
        map(
          (result) => result.data!.apis_kcp_io.v1alpha2.updateAPIBinding,
        ),
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

  public getCatalog(): Observable<CrossplaneCatalog | null> {
    return this.apollo
      .query<{
        crossplane_services_openmcp_cloud: {
          v1alpha1: { CrossplaneCatalog: CrossplaneCatalog | null };
        };
      }>({
        query: QUERY_CATALOG,
        fetchPolicy: 'network-only',
      })
      .pipe(
        map((result) => result.data!.crossplane_services_openmcp_cloud.v1alpha1.CrossplaneCatalog),
        catchError(() => of(null)),
      );
  }

  public createCrossplane(
    version: string,
    providers: { name: string; version: string }[],
  ): Observable<{ metadata: { name: string } }> {
    const providersInput = providers
      .map((p) => `{ name: "${p.name}", version: "${p.version}" }`)
      .join(', ');
    const mutation = gql`
      mutation {
        crossplane_services_openmcp_cloud {
          v1alpha1 {
            createCrossplane(
              object: {
                metadata: { name: "default" }
                spec: {
                  version: "${version}"
                  providers: [${providersInput}]
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
        crossplane_services_openmcp_cloud: {
          v1alpha1: {
            createCrossplane: { metadata: { name: string } };
          };
        };
      }>({
        mutation,
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

  public deleteCrossplane(): Observable<void> {
    const mutation = gql`
      mutation {
        crossplane_services_openmcp_cloud {
          v1alpha1 {
            deleteCrossplane(name: "default")
          }
        }
      }
    `;
    return this.apollo.mutate({ mutation }).pipe(map(() => void 0));
  }
}
