<?php

#[Route(defaults: ['_routeScope' => ['storefront']])]
class AddressController extends StorefrontController
{
    #[Route(path: '/account/address', name: 'frontend.account.address.page', options: ['seo' => false], defaults: ['_loginRequired' => true, '_noStore' => true], methods: ['GET'])]
    public function accountAddressOverview(Request $request, SalesChannelContext $context, CustomerEntity $customer): Response
    {
    }
}