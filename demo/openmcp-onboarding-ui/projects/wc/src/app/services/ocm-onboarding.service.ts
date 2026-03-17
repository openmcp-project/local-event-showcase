import { ApolloFactory, LuigiContext } from './apollo-factory';
import { Injectable, inject } from '@angular/core';
import { Apollo } from 'apollo-angular';
import { Observable, map, of, catchError } from 'rxjs';
import { gql } from '@apollo/client/core';

export interface OCMControllerStatus {
  metadata: { name: string };
  spec?: { version: string; chartVersion?: string };
  status?: { phase: string };
}

export interface OCMControllerEvent {
  type: 'ADDED' | 'MODIFIED' | 'DELETED';
  object: OCMControllerStatus;
}

const CHECK_OCM_CONTROLLER = gql`
  query {
    ocm_services_openmcp_cloud {
      v1alpha1 {
        OCMController(name: "default") {
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

const WATCH_OCM_CONTROLLER = gql`
  subscription {
    ocm_services_openmcp_cloud_v1alpha1_ocmcontroller(name: "default") {
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
export class OCMOnboardingService {
  private apolloFactory = inject(ApolloFactory);
  private apollo!: Apollo;

  public initialize(context: LuigiContext): void {
    this.apollo = this.apolloFactory.apollo(context);
  }

  public checkOCMController(): Observable<OCMControllerStatus | null> {
    return this.apollo
      .query<{
        ocm_services_openmcp_cloud: {
          v1alpha1: { OCMController: OCMControllerStatus | null };
        };
      }>({
        query: CHECK_OCM_CONTROLLER,
        fetchPolicy: 'network-only',
      })
      .pipe(
        map((result) => result.data!.ocm_services_openmcp_cloud.v1alpha1.OCMController),
        catchError((err) => {
          if (err.message?.includes('not found')) {
            return of(null);
          }
          throw err;
        }),
      );
  }

  public createOCMController(
    version: string,
    chartVersion?: string,
  ): Observable<{ metadata: { name: string } }> {
    const chartVersionInput = chartVersion ? `chartVersion: "${chartVersion}"` : '';
    const mutation = gql`
      mutation {
        ocm_services_openmcp_cloud {
          v1alpha1 {
            createOCMController(
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
        ocm_services_openmcp_cloud: {
          v1alpha1: {
            createOCMController: { metadata: { name: string } };
          };
        };
      }>({
        mutation,
      })
      .pipe(
        map((result) => result.data!.ocm_services_openmcp_cloud.v1alpha1.createOCMController),
      );
  }

  public watchOCMController(): Observable<OCMControllerEvent> {
    return this.apollo
      .subscribe<{
        ocm_services_openmcp_cloud_v1alpha1_ocmcontroller: OCMControllerEvent;
      }>({
        query: WATCH_OCM_CONTROLLER,
      })
      .pipe(
        map((result) => result.data!.ocm_services_openmcp_cloud_v1alpha1_ocmcontroller),
      );
  }

  public deleteOCMController(): Observable<void> {
    const mutation = gql`
      mutation {
        ocm_services_openmcp_cloud {
          v1alpha1 {
            deleteOCMController(name: "default")
          }
        }
      }
    `;
    return this.apollo.mutate({ mutation }).pipe(map(() => void 0));
  }
}
