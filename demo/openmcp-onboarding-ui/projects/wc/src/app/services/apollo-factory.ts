import { Injectable, NgZone, inject } from '@angular/core';
import { HttpHeaders } from '@angular/common/http';
import {
  type ApolloClientOptions,
  ApolloLink,
  Observable as ApolloObservable,
  FetchResult,
  InMemoryCache,
  Operation,
  split,
} from '@apollo/client/core';
import { setContext } from '@apollo/client/link/context';
import { getMainDefinition } from '@apollo/client/utilities';
import { Apollo } from 'apollo-angular';
import { HttpLink } from 'apollo-angular/http';
import { print } from 'graphql';
import { Client, ClientOptions, createClient } from 'graphql-sse';

export interface LuigiContext {
  token: string;
  portalContext: {
    crdGatewayApiUrl: string;
  };
  entityType?: string;
  entityName?: string;
  userId?: string;
  accountPath?: string;
}

class SSELink extends ApolloLink {
  private client: Client;

  constructor(options: ClientOptions) {
    super();
    this.client = createClient(options);
  }

  public override request(
    operation: Operation,
  ): ApolloObservable<FetchResult> {
    return new ApolloObservable((sink) => {
      return this.client.subscribe(
        { ...operation, query: print(operation.query) },
        {
          next: sink.next.bind(sink),
          complete: sink.complete.bind(sink),
          error: sink.error.bind(sink),
        },
      );
    });
  }
}

@Injectable({
  providedIn: 'root',
})
export class ApolloFactory {
  private httpLink = inject(HttpLink);
  private ngZone = inject(NgZone);

  public apollo(context: LuigiContext): Apollo {
    return new Apollo(this.ngZone, this.createApolloOptions(context));
  }

  private createApolloOptions(
    context: LuigiContext,
  ): ApolloClientOptions {
    const gatewayUrl = context.portalContext.crdGatewayApiUrl;

    const contextLink = setContext(() => ({
      uri: gatewayUrl,
      headers: new HttpHeaders({
        Authorization: `Bearer ${context.token}`,
        Accept: 'charset=utf-8',
      }),
    }));

    const splitClient = split(
      ({ query }) => {
        const definition = getMainDefinition(query);
        return (
          definition.kind === 'OperationDefinition' &&
          definition.operation === 'subscription'
        );
      },
      new SSELink({
        url: () => gatewayUrl,
        headers: () => ({
          Authorization: `Bearer ${context.token}`,
        }),
      }),
      this.httpLink.create({}),
    );

    const link = ApolloLink.from([contextLink, splitClient]);
    const cache = new InMemoryCache();

    return {
      link,
      cache,
    };
  }
}
