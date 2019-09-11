.. @jsii-pacmak:meta@ {"fingerprint":"bWSSzdBiB06WED+JFFPaDleK5rIKDaIMZEuIxMXItKA="}

experiments
===========

Reference
---------

.. tabs::

   .. group-tab:: C#

      View in `Nuget <https://www.nuget.org/packages/Acme.HelloPackage/1.0.0>`_

      **csproj**:

      .. code-block:: xml

         <PackageReference Include="Acme.HelloPackage" Version="1.0.0" />

      **dotnet**:

      .. code-block:: console

         dotnet add package Acme.HelloPackage --version 1.0.0

      **packages.config**:

      .. code-block:: xml

         <package id="Acme.HelloPackage" version="1.0.0" />


   .. group-tab:: Java

      View in `Maven Central <https://repo1.maven.org/maven2/com/acme/hello/hello-jsii/1.0.0/>`_

      **Apache Buildr**:

      .. code-block:: none

         'com.acme.hello:hello-jsii:jar:1.0.0'

      **Apache Ivy**:

      .. code-block:: xml

         <dependency groupId="com.acme.hello" name="hello-jsii" rev="1.0.0"/>

      **Apache Maven**:

      .. code-block:: xml

         <dependency>
           <groupId>com.acme.hello</groupId>
           <artifactId>hello-jsii</artifactId>
           <version>1.0.0</version>
         </dependency>

      **Gradle / Grails**:

      .. code-block:: none

         compile 'com.acme.hello:hello-jsii:1.0.0'

      **Groovy Grape**:

      .. code-block:: none

         @Grapes(
         @Grab(group='com.acme.hello', module='hello-jsii', version='1.0.0')
         )


   .. group-tab:: JavaScript

      View in `NPM <https://www.npmjs.com/package/experiments/v/1.0.0>`_

      **npm**:

      .. code-block:: console

         $ npm i experiments@1.0.0

      **package.json**:

      .. code-block:: js

         {
           "experiments": "^1.0.0"
         }

      **yarn**:

      .. code-block:: console

         $ yarn add experiments@1.0.0


   .. group-tab:: TypeScript

      View in `NPM <https://www.npmjs.com/package/experiments/v/1.0.0>`_

      **npm**:

      .. code-block:: console

         $ npm i experiments@1.0.0

      **package.json**:

      .. code-block:: js

         {
           "experiments": "^1.0.0"
         }

      **yarn**:

      .. code-block:: console

         $ yarn add experiments@1.0.0



.. py:module:: experiments

HelloJsii
^^^^^^^^^

.. py:class:: HelloJsii()

   **Language-specific names:**

   .. tabs::

      .. code-tab:: c#

         using Acme.HelloNamespace;

      .. code-tab:: java

         import com.acme.hello.HelloJsii;

      .. code-tab:: javascript

         const { HelloJsii } = require('experiments');

      .. code-tab:: typescript

         import { HelloJsii } from 'experiments';




   .. py:method:: baz(input) -> string or experiments.IOutput

      :param input: 
      :type input: number or :py:class:`~experiments.IOutput`\ 
      :rtype: string or :py:class:`~experiments.IOutput`\ 


   .. py:attribute:: strings

      :type: string[]


IOutput (interface)
^^^^^^^^^^^^^^^^^^^

.. py:class:: IOutput

   **Language-specific names:**

   .. tabs::

      .. code-tab:: c#

         using Acme.HelloNamespace;

      .. code-tab:: java

         import com.acme.hello.IOutput;

      .. code-tab:: javascript

         // IOutput is an interface

      .. code-tab:: typescript

         import { IOutput } from 'experiments';





   .. py:method:: val() -> any

      :rtype: any
      :abstract: Yes


