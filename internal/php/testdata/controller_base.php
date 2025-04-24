<?php

#[Route(path: '/api')]
class ApiController
{
    #[Route(name: 'foo', path: '/foo')]
    public function test()
    {

    }
}