<?php

namespace App\Controller;

// Group use statement with namespace prefix
use Symfony\Component\{
    HttpFoundation\Request,
    HttpFoundation\Response,
    HttpKernel\Kernel
};

// Group use statement with aliases
use Doctrine\DBAL\{
    Connection as DbConnection,
    Statement as DbStatement
};

// Nested group use statement
use Shopware\Core\Framework\Context;
use Shopware\Core\Framework\DataAbstractionLayer\EntityRepository;
use Shopware\Core\Framework\DataAbstractionLayer\Search\Criteria;
use Shopware\Core\Framework\DataAbstractionLayer\Search\Filter\EqualsFilter;

class TestController
{
    private Request $request;
    private Response $response;
    private Kernel $kernel;
    private DbConnection $connection;
    private DbStatement $statement;
    private Context $context;
    private EntityRepository $repository;
    private Criteria $criteria;
    private EqualsFilter $filter;
}
