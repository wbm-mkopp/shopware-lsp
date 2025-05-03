<?php

namespace Shopware\Core\Content\Product\Advanced;

// Group use statements
use Symfony\Component\{
    HttpFoundation\Request,
    HttpFoundation\Response,
    HttpKernel\Kernel
};

// Aliased use statements
use Doctrine\DBAL\Connection as DbConnection;
use Shopware\Core\Framework\DataAbstractionLayer\EntityRepository as Repository;
use Shopware\Core\Framework\Context;

class AdvancedProductTest
{
    private Request $request;
    private Response $response;
    private Kernel $kernel;
    private DbConnection $connection;
    private Repository $productRepository;
    private Context $context;
    
    // Method with union type return (PHP 8.0+)
    public function getRequestOrResponse(): Request|Response
    {
        return $this->request;
    }
    
    // Method with intersection type parameter (PHP 8.1+)
    public function processConnection(DbConnection&Repository $connection): void
    {
        // Implementation
    }
    
    // Method with nullable type
    public function getOptionalKernel(): ?Kernel
    {
        return $this->kernel;
    }
    
    // Method with mixed return type
    public function getData(): mixed
    {
        return [];
    }
}
